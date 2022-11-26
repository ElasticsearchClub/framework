/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"strings"
	"sync"
	"time"
)

type Context struct {
	//Metadata   map[string]interface{} `config:"metadata" json:"metadata"`
	NextOffset string `config:"next_offset" json:"next_offset"`
	InitOffset string `config:"init_offset" json:"init_offset"`
}

func (c *Context) ToString() string {
	return fmt.Sprintf("%v->%v",c.InitOffset,c.NextOffset)
}

type Message struct {
	Timestamp  int64  `config:"timestamp" json:"timestamp"`
	Offset     string `config:"offset" json:"offset"`           //current offset
	NextOffset string `config:"next_offset" json:"next_offset"` //offset for next message
	Size       int  `config:"size" json:"size"`
	Data       []byte `config:"data" json:"data"`
}

type QueueAPI interface {

	AdvancedQueueAPI

	Name() string
	Init(string) error
	Push(string, []byte) error
	Pop(string, time.Duration) (data []byte, timeout bool)
	//segment means the sequence id of the queue, offset is within the segment, count means how many messages will be fetching
	Close(string) error
	Depth(string) int64

	Consume(queue *QueueConfig, consumer *ConsumerConfig, offset string) (*Context, []Message, bool, error)
	LatestOffset(string) string

	GetQueues() []string
}

type AdvancedQueueAPI interface {
	AcquireConsumer(qconfig *QueueConfig,consumer *ConsumerConfig, segment, readPos int64) (ConsumerAPI,error)
}

type ConsumerAPI interface {
	Close()error
	ResetOffset(part, readPos int64) (err error)
	FetchMessages(numOfMessages int) (ctx *Context, messages []Message, isTimeout bool, err error)
}

var defaultHandler QueueAPI
var handlers map[string]QueueAPI = map[string]QueueAPI{}

type QueueConfig struct {
	Source string      `config:"source" json:"source,omitempty"`
	Id     string      `config:"id" json:"id,omitempty"`     //uuid for each queue
	Name   string      `config:"name" json:"name,omitempty"` //unique name of each queue
	Codec  string      `config:"codec" json:"codec,omitempty"`
	Type   string      `config:"type" json:"type,omitempty"`
	Labels util.MapStr `config:"label" json:"label,omitempty"`
}

var queueConfigPool = sync.Pool{
	New: func() interface{} {
		return new(QueueConfig)
	},
}

func AcquireQueueConfig() *QueueConfig {

	cfg := queueConfigPool.Get().(*QueueConfig)
	cfg.Id = ""
	cfg.Name = ""
	cfg.Type = ""
	cfg.Codec = ""
	cfg.Source = ""
	cfg.Labels = util.MapStr{}
	return cfg
}

func ReturnQueueConfig(cfg *QueueConfig) {
	cfg.Id = ""
	cfg.Name = ""
	cfg.Type = ""
	cfg.Codec = ""
	cfg.Source = ""
	cfg.Labels = nil
	queueConfigPool.Put(cfg)
}

type ConsumerConfig struct {
	Source     string `config:"source" json:"source,omitempty"`
	Id         string `config:"id" json:"id,omitempty"` //uuid for each queue
	Group      string `config:"group" json:"group,omitempty"`
	Name       string `config:"name" json:"name,omitempty"`
	//AutoReset  string `config:"auto_offset_reset" json:"auto_offset_reset,omitempty"`
	//AutoCommit bool   `config:"auto_commit" json:"auto_commit,omitempty"`

	FetchMinBytes    int `config:"fetch_min_bytes" json:"fetch_min_bytes,omitempty"`
	FetchMaxBytes    int `config:"fetch_max_bytes" json:"fetch_max_bytes,omitempty"`
	FetchMaxMessages int   `config:"fetch_max_messages" json:"fetch_max_messages,omitempty"`
	FetchMaxWaitMs   int64   `config:"fetch_max_wait_ms" json:"fetch_max_wait_ms,omitempty"`
	fetchMaxWaitMs   time.Duration
}

func (cfg *ConsumerConfig) Key() string {
	return fmt.Sprintf("%v-%v", cfg.Group, cfg.Name)
}

func (cfg *ConsumerConfig) GetFetchMaxWaitMs() time.Duration {
	if cfg.fetchMaxWaitMs.Milliseconds() > 0 {
		return cfg.fetchMaxWaitMs
	}

	cfg.fetchMaxWaitMs = time.Duration(cfg.FetchMaxWaitMs) * time.Millisecond
	return cfg.fetchMaxWaitMs
}

func (cfg *ConsumerConfig) String() string {
	return fmt.Sprintf("group:%v,name:%v,id:%v,source:%v",cfg.Group,cfg.Name,cfg.Id,cfg.Source)
}

func getHandler(k *QueueConfig) QueueAPI {
	handler, ok := handlers[k.Id]
	if handler != nil && ok {
		return handler
	}
	handler, ok = adapters[k.Type]
	if ok && handler != nil {
		return handler
	}
	if defaultHandler==nil{
		panic(errors.New("no queue handler was found"))
	}
	return defaultHandler
}

func Push(k *QueueConfig, v []byte) error {
	var err error = nil
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler := getHandler(k)
	if handler != nil {
		err = handler.Push(k.Id, v)
		if err == nil {
			stats.Increment("queue", k.Id, "push")
			return nil
		}
		stats.Increment("queue", k.Id, "push_error")
		return err
	}
	panic(errors.Errorf("handler for [%v] is not registered",k))
}

//var pauseMsg = errors.New("queue was paused to read")

var configs = map[string]*QueueConfig{}
var idConfigs = map[string]*QueueConfig{}
var cfgLock = sync.RWMutex{}
var consumerCfgLock = sync.RWMutex{}
var existsErr = errors.New("config exists")

func RegisterConfig(queueKey string, cfg *QueueConfig) (bool, error) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	_, ok := configs[queueKey]
	if ok {
		return true, existsErr
	} else {
		//init empty id
		if cfg.Id == "" {
			cfg.Id = util.GetUUID()
		}
		idConfigs[cfg.Id] = cfg
		configs[queueKey] = cfg

		//async notify
		go func() {
			for _, f := range queueConfigListener {
				f(cfg)
			}
		}()

		return false, nil
	}
}

const consumerBucket = "queue_consumers"

func RegisterConsumer(queueID string, consumer *ConsumerConfig) (bool, error) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	ok, _ := kv.ExistsKey(consumerBucket, queueIDBytes)

	cfgs := map[string]*ConsumerConfig{}
	if ok {
		data, err := kv.GetValue(consumerBucket, queueIDBytes)
		if err != nil {
			panic(err)
		}
		err = util.FromJSONBytes(data, &cfgs)
		if err != nil {
			panic(err)
		}
	}

	cfgs[consumer.Key()] = consumer

	kv.AddValue(consumerBucket, queueIDBytes, util.MustToJSONBytes(cfgs))

	//async notify
	go func() {
		for _, f := range consumerConfigListener {
			f(queueID, cfgs)
		}
	}()

	return false, nil
}

func GetConsumerConfig(queueID, group, name string) (*ConsumerConfig, bool) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	cfgs := map[string]*ConsumerConfig{}
	data, err := kv.GetValue(consumerBucket, queueIDBytes)
	if err != nil {
		panic(err)
	}
	err = util.FromJSONBytes(data, &cfgs)
	if err != nil {
		panic(err)
	}
	if cfgs != nil {
		x, ok := cfgs[fmt.Sprintf("%v-%v", group, name)]
		return x, ok
	}

	return nil, false
}

func NewConsumerConfig(group, name string) *ConsumerConfig {
	cfg := &ConsumerConfig{
		FetchMinBytes:    1,
		FetchMaxBytes:    10 * 1024 * 1024,
		FetchMaxMessages: 500,
		FetchMaxWaitMs:   10000,
	}
	cfg.Id = util.GetUUID()
	cfg.Source = "dynamic"
	cfg.Group = group
	cfg.Name = name
	return cfg
}

func GetOrInitConsumerConfig(queueID, group, name string) *ConsumerConfig {
	cfg, exists := GetConsumerConfig(queueID, group, name)
	if !exists {
		cfg = &ConsumerConfig{
			FetchMinBytes:    1,
			FetchMaxBytes:    10 * 1024 * 1024,
			FetchMaxMessages: 500,
			FetchMaxWaitMs:   10000,
		}
		cfg.Id = util.GetUUID()
		cfg.Source = "dynamic"
		cfg.Group = group
		cfg.Name = name
		RegisterConsumer(queueID, cfg)
	}
	return cfg
}

func IsConfigExists(key string) bool {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	_, ok := configs[key]
	return ok
}

func GetOrInitConfig(key string) *QueueConfig {
	cfg, exists := SmartGetConfig(key)
	if !exists {
		cfgLock.Lock()
		cfg, exists = configs[key]
		cfgLock.Unlock()
		if !exists {
			cfg = &QueueConfig{}
			cfg.Id = util.GetUUID()
			cfg.Name = key
			cfg.Source = "dynamic"
			RegisterConfig(key, cfg)
		}
	}
	return cfg
}

func SmartGetConfig(keyOrID string) (*QueueConfig, bool) {
	q,ok:=GetConfigByKey(keyOrID)
	if !ok{
		q,ok=GetConfigByUUID(keyOrID)
	}
	return q,ok
}

func GetConfigByKey(key string) (*QueueConfig, bool) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	v, ok := configs[key]
	return v, ok
}

func GetConfigByUUID(id string) (*QueueConfig, bool) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	v, ok := idConfigs[id]
	return v, ok
}

func GetConsumerConfigsByQueueID(queueID string) (map[string]*ConsumerConfig, bool) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()

	queueIDBytes := util.UnsafeStringToBytes(queueID)
	cfgs := map[string]*ConsumerConfig{}
	data, err := kv.GetValue(consumerBucket, queueIDBytes)
	if err != nil {
		panic(err)
	}
	//TODO optimize performance
	err = util.FromJSONBytes(data, &cfgs)
	if err != nil {
		panic(err)
	}

	if cfgs != nil {
		return cfgs, len(cfgs) > 0
	}

	return nil, false
}

func GetConsumerConfigID(queueID, consumerID string) (*ConsumerConfig, bool) {
	m, ok := GetConsumerConfigsByQueueID(queueID)
	if ok {
		for _, v := range m {
			if v.Id == consumerID {
				return v, true
			}
		}
	}
	return nil, false
}

func GetAllConfigBytes() []byte {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	return util.MustToJSONBytes(configs)
}

func GetAllConfigs() map[string]*QueueConfig {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	return configs
}

func Pop(k *QueueConfig) ([]byte, error) {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)
	if handler != nil {
		//if pausedReadQueue.Contains(k) {
		//	return nil, pauseMsg
		//}

		o, timeout := handler.Pop(k.Id, -1)
		if !timeout {
			stats.Increment("queue", k.Id, "pop")
			return o, nil
		}
		stats.Increment("queue", k.Id, "pop_timeout")
		return o, errors.New("timeout")
	}
	panic(errors.New("handler is not registered"))
}

//consumer.Name,offset,processor.config.Consumer.FetchMaxMessages,time.Millisecond*time.Duration(processor.config.Consumer.FetchMaxWaitMs)
func Consume(k *QueueConfig, consumer *ConsumerConfig, offset string) (ctx *Context, messages []Message, isTimeout bool, err error) {
	//,offsetStr string,count int,timeout time.Duration
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	messages = []Message{}
	handler := getHandler(k)
	if handler != nil {
		//if pausedReadQueue.Contains(k) {
		//	return ctx,messages,isTimeout, pauseMsg
		//}

		ctx, messages, isTimeout, err = handler.Consume(k, consumer, offset)

		if !isTimeout {
			stats.Increment("queue", k.Id, "consume")
			return ctx, messages, isTimeout, err
		}
		stats.Increment("queue", k.Id, "consume_timeout")
		return ctx, messages, isTimeout, err
	}
	panic(errors.New("handler is not registered"))
}

func ConvertOffset(offsetStr string) (int64, int64) {
	data := strings.Split(offsetStr, ",")
	if len(data) != 2 {
		panic(errors.Errorf("invalid offset: %v", offsetStr))
	}
	var segment, offset int64
	segment, _ = util.ToInt64(data[0])
	offset, _ = util.ToInt64(data[1])
	return segment, offset
}

func AcquireConsumer(k *QueueConfig, consumer *ConsumerConfig, offset string)  (ConsumerAPI,error) {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler := getHandler(k)
	if handler != nil {
		segment,pos:=ConvertOffset(offset)
		return handler.AcquireConsumer(k,consumer,segment,pos)
	}
	panic(errors.New("handler is not registered"))
}


func PopTimeout(k *QueueConfig, timeoutInSeconds time.Duration) (data []byte, timeout bool, err error) {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	if timeoutInSeconds < 1 {
		timeoutInSeconds = 5
	}

	handler := getHandler(k)

	if handler != nil {
		//if pausedReadQueue.Contains(k) {
		//	return nil, false, pauseMsg
		//}

		o, timeout := handler.Pop(k.Id, timeoutInSeconds)
		if !timeout {
			stats.Increment("queue", k.Id, "pop")
		}
		stats.Increment("queue", k.Id, "pop_timeout")
		return o, timeout, nil
	}
	panic(errors.New("handler is not registered"))
}

func Close(k *QueueConfig) error {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)
	if handler != nil {
		o := handler.Close(k.Id)
		stats.Increment("queue", k.Id, "close")
		return o
	}
	stats.Increment("queue", k.Id, "close_error")
	panic(errors.New("handler is not closed"))
}

func getCommitKey(k *QueueConfig, consumer *ConsumerConfig) string {
	return fmt.Sprintf("%v-%v", k.Id, consumer.Id)
}

const consumerOffsetBucket = "queue_consumer_commit_offset"

func GetEarlierOffsetStrByQueueID(queueID string) string {
	_, seg, pos := GetEarlierOffsetByQueueID(queueID)
	offset := fmt.Sprintf("%v,%v", seg, pos)
	return offset
}

func GetEarlierOffsetByQueueID(queueID string) (consumerSize int, segment int64, pos int64) {
	q, ok := GetConfigByUUID(queueID)
	if !ok {
		q, ok = GetConfigByKey(queueID)
		if !ok {
			panic(errors.Errorf("queue [%v] was not found", queueID))
		}

		oldID := queueID
		queueID = q.Id

		if global.Env().IsDebug {
			log.Tracef("[%v] is not a valid uuid, found as key, continue as [%v]", oldID, queueID)
		}
	}
	consumers, ok := GetConsumerConfigsByQueueID(queueID)
	if !ok {
		return 0, 0, 0
	}
	var iPart int64
	var iPos int64
	var init = true
	for _, v := range consumers {
		offset, err := GetOffset(q, v)
		if err == nil {
			str := strings.Split(offset, ",")
			if len(str) == 2 {
				part, err := util.ToInt64(str[0])
				if err == nil {
					pos, err := util.ToInt64(str[1])
					if err == nil {
						if init {
							iPart = part
							iPos = pos
							init = false
						} else {
							if pos < iPos {
								iPos = pos
							}
							if part < iPart {
								iPart = part
							}
						}
					}
				}
			}
		}
	}
	return len(consumers), iPart, iPos
}

func GetLatestOffsetByQueueID(queueID string) (consumerSize int, segment int64, pos int64) {
	q, ok := GetConfigByUUID(queueID)
	if !ok {
		q, ok = GetConfigByKey(queueID)
		if !ok {
			panic(errors.Errorf("queue [%v] was not found", queueID))
		}

		oldID := queueID
		queueID = q.Id

		if global.Env().IsDebug {
			log.Tracef("[%v] is not a valid uuid, found as key, continue as [%v]", oldID, queueID)
		}
	}
	consumers, ok := GetConsumerConfigsByQueueID(queueID)
	if !ok {
		return 0, 0, 0
	}
	var iPart int64
	var iPos int64
	var init = true
	for _, v := range consumers {
		offset, err := GetOffset(q, v)
		if err == nil {
			str := strings.Split(offset, ",")
			if len(str) == 2 {
				part, err := util.ToInt64(str[0])
				if err == nil {
					pos, err := util.ToInt64(str[1])
					if err == nil {
						if init {
							iPart = part
							iPos = pos
							init = false
						} else {
							if pos > iPos {
								iPos = pos
							}
							if part > iPart {
								iPart = part
							}
						}
					}
				}
			}
		}
	}
	return len(consumers), iPart, iPos
}

func GetOffset(k *QueueConfig, consumer *ConsumerConfig) (string, error) {

	bytes, err := kv.GetValue(consumerOffsetBucket, util.UnsafeStringToBytes(getCommitKey(k, consumer)))
	if err != nil {
		log.Error(err)
	}

	if bytes != nil && len(bytes) > 0 {
		return string(bytes), nil
	}

	return "0,0", nil
}

func DeleteOffset(k *QueueConfig, consumer *ConsumerConfig) error {
	return kv.DeleteKey(consumerOffsetBucket, util.UnsafeStringToBytes(getCommitKey(k, consumer)))
}

func CommitOffset(k *QueueConfig, consumer *ConsumerConfig, offset string) (bool, error) {

	if global.Env().IsDebug {
		log.Tracef("queue [%v] [%v][%v] commit offset:%v", k.Id, consumer.Group, consumer.Name, offset)
	}

	err := kv.AddValue(consumerOffsetBucket, util.UnsafeStringToBytes(getCommitKey(k, consumer)), []byte(offset))
	if err != nil {
		return false, err
	}

	return true, nil
}

func Depth(k *QueueConfig) int64 {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)
	if handler != nil {
		o := handler.Depth(k.Id)
		stats.Increment("queue", k.Id, "call_depth")
		return o
	}
	panic(errors.New("handler is not registered"))
}

func HasLag(k *QueueConfig) bool {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)

	if handler != nil {

		latestProduceOffset := LatestOffset(k)
		offset := GetEarlierOffsetStrByQueueID(k.Id)

		if latestProduceOffset != offset {
			return true
		}

		stats.Increment("queue", k.Id, "check_lag")
		return false
	}

	panic(errors.New("handler is not registered"))
}

func ConsumerHasLag(k *QueueConfig,c *ConsumerConfig) bool {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)

	if handler != nil {
		latestProduceOffset := LatestOffset(k)
		offset,err := GetOffset(k,c)
		if err!=nil{
			panic(err)
		}

		if latestProduceOffset != offset {
			return true
		}

		stats.Increment("queue", k.Id, "check_consumer_lag")
		return false
	}

	panic(errors.New("handler is not registered"))
}

func LatestOffset(k *QueueConfig) string {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}

	handler := getHandler(k)
	if handler != nil {
		o := handler.LatestOffset(k.Id)
		return o
	}
	panic(errors.New("handler is not registered"))
}

func GetQueues() map[string][]string {
	results := map[string][]string{}
	for q, handler := range adapters {
		result := []string{}
		if handler != nil {
			o := handler.GetQueues()
			stats.Increment("queue", q, "get_queues")
			result = append(result, o...)
			results[q] = result
		}
	}
	return results
}

type QueueSelector struct {
	Labels map[string]interface{} `config:"labels,omitempty"`
	Ids    []string               `config:"ids,omitempty"`
	Keys   []string               `config:"keys,omitempty"`
}

func (s *QueueSelector) ToString() string {
	return fmt.Sprintf("ids:%v, keys:%v, labels:%v", s.Ids, s.Keys, s.Labels)
}

func GetConfigBySelector(selector *QueueSelector) []*QueueConfig {
	cfgs := []*QueueConfig{}
	if selector != nil {
		if len(selector.Ids) > 0 {
			for _, id := range selector.Ids {
				cfg, ok := GetConfigByUUID(id)
				if ok {
					cfgs = append(cfgs, cfg)
				}
			}
		}

		if len(selector.Keys) > 0 {
			for _, key := range selector.Keys {
				cfg, ok := GetConfigByKey(key)
				if ok {
					cfgs = append(cfgs, cfg)
				}
			}
		}

		if len(selector.Labels) > 0 {
			cfgs1 := GetConfigByLabels(selector.Labels)
			if cfgs1 != nil {
				cfgs = append(cfgs, cfgs1...)
			}
		}
	}

	log.Tracef("selector:%v, get queues: %v",selector,cfgs)

	return cfgs
}

func GetConfigByLabels(labels map[string]interface{}) []*QueueConfig {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	cfgs := []*QueueConfig{}

	for _, v := range configs {
		notMatch := false
		for x, y := range labels {
			z, ok := v.Labels[x]
			if ok {
				if util.ToString(z) != util.ToString(y) {
					notMatch = true
				}
			} else {
				notMatch = true
			}
		}
		if !notMatch {
			cfgs = append(cfgs, v)
		}
	}
	return cfgs
}

var pausedReadQueue = hashset.New()
var pauseChan map[string]chan bool = map[string]chan bool{}
var pauseCount = map[string]int{}
var pauseLock sync.Mutex

func PauseRead(k string) {
	pauseLock.Lock()
	defer pauseLock.Unlock()
	pauseCount[k] = 0
	pauseChan[k] = make(chan bool)
	pausedReadQueue.Add(k)
}

func ResumeRead(k string) {
	pauseLock.Lock()
	defer pauseLock.Unlock()
	pausedReadQueue.Remove(k)
	size := pauseCount[k]
	for i := 0; i < size; i++ {
		pauseChan[k] <- true
	}
	log.Debugf("queue: %s was resumed, signal: %v", k, size)
}

var adapters map[string]QueueAPI = map[string]QueueAPI{}

func RegisterDefaultHandler(h QueueAPI) {
	defaultHandler = h
}

func IniQueue(k *QueueConfig) {
	if k == nil || k.Id == "" {
		panic(errors.New("queue name can't be nil"))
	}
	handler := getHandler(k)
	handlers[k.Id] = handler
	err := handler.Init(k.Id)
	if err != nil {
		panic(err)
	}
}

func Register(name string, h QueueAPI) {
	_, ok := adapters[name]
	if ok {
		panic(errors.Errorf("queue adapter with same name: %v already exists", name))
	}

	adapters[name] = h
	log.Debug("register queue adapter: ", name)
}

//TODO only update specify event, func(queueID)
var queueConfigListener = []func(cfg *QueueConfig){}

func RegisterQueueConfigChangeListener(l func(cfg *QueueConfig)) {
	cfgLock.Lock()
	defer cfgLock.Unlock()
	queueConfigListener = append(queueConfigListener, l)
}

var consumerConfigListener = []func(id string, configs map[string]*ConsumerConfig){}

func RegisterConsumerConfigChangeListener(l func(id string, configs map[string]*ConsumerConfig)) {
	consumerCfgLock.Lock()
	defer consumerCfgLock.Unlock()
	consumerConfigListener = append(consumerConfigListener, l)
}
