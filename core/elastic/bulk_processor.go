package elastic

import (
	"bufio"
	"bytes"
	"fmt"
	log "github.com/cihub/seelog"
	pool "github.com/libp2p/go-buffer-pool"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"net/http"
	"strings"
	"time"
)

var NEWLINEBYTES = []byte("\n")
var BulkDocBuffer pool.BufferPool

func WalkBulkRequests(safetyParse bool, data []byte, docBuff []byte, eachLineFunc func(eachLine []byte) (skipNextLine bool), metaFunc func(metaBytes []byte, actionStr, index, typeName, id,routing string) (err error), payloadFunc func(payloadBytes []byte)) (int, error) {

	nextIsMeta := true
	skipNextLineProcessing := false
	var docCount = 0

START:

	if safetyParse {
		lines := bytes.Split(data, NEWLINEBYTES)
		//reset
		nextIsMeta = true
		skipNextLineProcessing = false
		docCount = 0
		for i, line := range lines {

			bytesCount := len(line)
			if line == nil || bytesCount <= 0 {
				if global.Env().IsDebug {
					log.Tracef("invalid line, continue, [%v/%v] [%v]\n%v", i, len(lines), string(line), util.PrintStringByteLines(lines))
				}
				continue
			}

			if eachLineFunc != nil {
				skipNextLineProcessing = eachLineFunc(line)
			}

			if skipNextLineProcessing {
				skipNextLineProcessing = false
				nextIsMeta = true
				log.Debug("skip body processing")
				continue
			}

			if nextIsMeta {
				nextIsMeta = false
				var actionStr string
				var index string
				var typeName string
				var id string
				var routing string
				actionStr, index, typeName, id,routing = ParseActionMeta(line)

				err := metaFunc(line, actionStr, index, typeName, id,routing)
				if err != nil {
					log.Debug(err)
					return docCount, err
				}

				docCount++

				if actionStr == ActionDelete {
					nextIsMeta = true
					payloadFunc(nil)
				}
			} else {
				nextIsMeta = true
				payloadFunc(line)
			}
		}
	}

	if !safetyParse {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Split(util.GetSplitFunc(NEWLINEBYTES))

		sizeOfDocBuffer := len(docBuff)
		if sizeOfDocBuffer > 0 {
			if sizeOfDocBuffer < 1024 {
				log.Debug("doc buffer size maybe too small,", sizeOfDocBuffer)
			}
			scanner.Buffer(docBuff, sizeOfDocBuffer)
		}

		processedBytesCount := 0
		for scanner.Scan() {
			scannedByte := scanner.Bytes()
			bytesCount := len(scannedByte)
			processedBytesCount += bytesCount
			if scannedByte == nil || bytesCount <= 0 {
				log.Debug("invalid scanned byte, continue")
				continue
			}

			if eachLineFunc != nil {
				skipNextLineProcessing = eachLineFunc(scannedByte)
			}

			if skipNextLineProcessing {
				skipNextLineProcessing = false
				nextIsMeta = true
				log.Debug("skip body processing")
				continue
			}

			if nextIsMeta {

				nextIsMeta = false

				//TODO improve poor performance
				var actionStr string
				var index string
				var typeName string
				var id string
				var routing string
				actionStr, index, typeName, id,routing = ParseActionMeta(scannedByte)

				err := metaFunc(scannedByte, actionStr, index, typeName, id,routing)
				if err != nil {
					if global.Env().IsDebug {
						log.Error(err)
					}
					return docCount, err
				}

				docCount++

				if actionStr == ActionDelete {
					nextIsMeta = true
					payloadFunc(nil)
				}
			} else {
				nextIsMeta = true
				payloadFunc(scannedByte)
			}
		}

		if processedBytesCount+sizeOfDocBuffer <= len(data) {
			log.Warn("bulk requests was not fully processed,", processedBytesCount, "/", len(data), ", you may need to increase `doc_buffer_size`, re-processing with memory inefficient way now")
			return 0, errors.New("documents too big, skip processing")
			safetyParse = true
			goto START
		}
	}

	if global.Env().IsDebug {
		log.Tracef("total [%v] operations in bulk requests", docCount)
	}

	return docCount, nil
}

func ParseUrlLevelBulkMeta(pathStr string) (urlLevelIndex, urlLevelType string) {

	if !util.SuffixStr(pathStr, "_bulk") {
		return urlLevelIndex, urlLevelType
	}

	if !util.PrefixStr(pathStr, "/") {
		return urlLevelIndex, urlLevelType
	}

	if strings.Index(pathStr, "//") >= 0 {
		pathStr = strings.ReplaceAll(pathStr, "//", "/")
	}

	if strings.LastIndex(pathStr, "/") == 0 {
		return urlLevelIndex, urlLevelType
	}

	pathArray := strings.Split(pathStr, "/")

	switch len(pathArray) {
	case 4:
		urlLevelIndex = pathArray[1]
		urlLevelType = pathArray[2]
		break
	case 3:
		urlLevelIndex = pathArray[1]
		break
	}

	return urlLevelIndex, urlLevelType
}

type BulkProcessorConfig struct {
	bulkSizeInByte int

	BulkSizeInKb     int `config:"batch_size_in_kb,omitempty"`
	BulkSizeInMb     int `config:"batch_size_in_mb,omitempty"`
	BulkMaxDocsCount int `config:"batch_size_in_docs,omitempty"`

	Compress                bool   `config:"compress"`
	RetryDelayInSeconds     int    `config:"retry_delay_in_seconds"`
	RejectDelayInSeconds    int    `config:"reject_retry_delay_in_seconds"`
	MaxRejectRetryTimes     int    `config:"max_reject_retry_times"`
	MaxRetryTimes           int    `config:"max_retry_times"`
	RequestTimeoutInSecond  int    `config:"request_timeout_in_second"`
	InvalidRequestsQueue    string `config:"invalid_queue"`
	DeadletterRequestsQueue string `config:"dead_letter_queue"`

	SafetyParse             bool   `config:"safety_parse"`

	IncludeBusyRequestsToFailureQueue bool `config:"include_busy_requests_to_failure_queue"`

	DocBufferSize           int    `config:"doc_buffer_size"`
}

func (this *BulkProcessorConfig) GetBulkSizeInBytes() int {

	this.bulkSizeInByte = 1048576 * this.BulkSizeInMb
	if this.BulkSizeInKb > 0 {
		this.bulkSizeInByte = 1024 * this.BulkSizeInKb
	}
	if this.bulkSizeInByte <= 0 {
		this.bulkSizeInByte = 10 * 1024 * 1024
	}
	return this.bulkSizeInByte
}

var DefaultBulkProcessorConfig = BulkProcessorConfig{
	BulkMaxDocsCount:        1000,
	BulkSizeInMb:            10,
	Compress:                false,
	RetryDelayInSeconds:     1,
	RejectDelayInSeconds:    1,
	MaxRejectRetryTimes:     60,
	MaxRetryTimes:           10,
	DeadletterRequestsQueue: "dead_letter_queue",

	SafetyParse:             true,
	IncludeBusyRequestsToFailureQueue:         true,

	DocBufferSize:           256 * 1024,
	RequestTimeoutInSecond:  60,
}

type BulkProcessor struct {
	Config BulkProcessorConfig
}

type API_STATUS string

func (joint *BulkProcessor) Bulk(tag string, metadata *ElasticsearchMetadata, host string, buffer *BulkBuffer) (continueNext bool, err error) {

	if buffer == nil || buffer.GetMessageSize() == 0 {
		return true, errors.New("invalid bulk requests, message is nil")
	}

	host = metadata.GetActivePreferredHost(host)

	if host==""{
		panic("invalid host")
	}

	httpClient := metadata.GetHttpClient(host)

	var url string
	if metadata.IsTLS() {
		url = fmt.Sprintf("https://%s/_bulk", host)
	} else {
		url = fmt.Sprintf("http://%s/_bulk", host)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)   // <- do not forget to release
	defer fasthttp.ReleaseResponse(resp) // <- do not forget to release

	req.SetRequestURI(url)
	//req.URI().Update(url)

	req.Header.SetMethod(http.MethodPost)
	req.Header.SetUserAgent("_bulk")
	req.Header.SetContentType("application/x-ndjson")

	if metadata.Config.BasicAuth != nil {
		req.URI().SetUsername(metadata.Config.BasicAuth.Username)
		req.URI().SetPassword(metadata.Config.BasicAuth.Password)
	}

	//acceptGzipped := req.AcceptGzippedResponse()
	//compressed := false

	// handle last \n
	data := buffer.GetMessageBytes()
	if !util.IsBytesEndingWith(&data,NEWLINEBYTES){
		if !util.BytesHasPrefix(buffer.GetMessageBytes(),NEWLINEBYTES){
			buffer.Write(NEWLINEBYTES)
			data=buffer.GetMessageBytes()
		}
	}

	if !req.IsGzipped() && joint.Config.Compress {

		_, err := fasthttp.WriteGzipLevel(req.BodyWriter(), data, fasthttp.CompressBestSpeed)
		if err != nil {
			panic(err)
		}

		//TODO handle response, if client not support gzip, return raw body
		req.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
		req.Header.Set(fasthttp.HeaderContentEncoding, "gzip")
		//compressed = true

	} else {
		req.SetBody(data)
		//req.SetRawBody(data)
	}

	//if req.GetBodyLength() <= 0 {
	//	panic(errors.Error("request body is zero,", len(data), ",is compress:", joint.Config.Compress))
	//}

	// modify schema，align with elasticsearch's schema
	orignalSchema := string(req.URI().Scheme())
	orignalHost := string(req.URI().Host())

	if host!=""&&req.Host()==nil||string(req.Host())!=orignalHost{
		req.Header.SetHost(host)
	}

	if metadata.GetSchema() != orignalSchema {
		req.URI().SetScheme(metadata.GetSchema())
	}

	retryTimes := 0
	nonRetryableItems := AcquireBulkBuffer()
	retryableItems := AcquireBulkBuffer()
	successItems := AcquireBulkBuffer()

	defer ReturnBulkBuffer(nonRetryableItems)
	defer ReturnBulkBuffer(retryableItems)
	defer ReturnBulkBuffer(successItems)

DO:

	if req.GetBodyLength() <= 0 {
		panic(errors.Error("request body is zero,", len(data), ",is compress:", joint.Config.Compress))
	}
	
	metadata.CheckNodeTrafficThrottle(util.UnsafeBytesToString(req.Header.Host()), 1, req.GetRequestLength(), 0)
	
	//execute
	err = httpClient.DoTimeout(req, resp, time.Duration(joint.Config.RequestTimeoutInSecond)*time.Second)

	if err != nil {
		if rate.GetRateLimiter(metadata.Config.ID, host+"5xx_on_error", 1, 1, 5*time.Second).Allow() {
			log.Error("status:", resp.StatusCode(), ",", host, ",", err, " ", util.SubString(util.UnsafeBytesToString(resp.GetRawBody()), 0, 256))
			time.Sleep(2 * time.Second)
		}
		return false, err
	}
	
	////restore body and header
	//if !acceptGzipped && compressed {
	//
	//	body := resp.GetRawBody()
	//
	//	resp.SwapBody(body)
	//
	//	resp.Header.Del(fasthttp.HeaderContentEncoding)
	//	resp.Header.Del(fasthttp.HeaderContentEncoding2)
	//
	//}

	//restore schema
	req.URI().SetScheme(orignalSchema)
	req.SetHost(orignalHost)

	if resp == nil {
		if global.Env().IsDebug {
			log.Error(err)
		}
		return false, err
	}
	
	// Do we need to decompress the response?
	var resbody = resp.GetRawBody()
	
	if global.Env().IsDebug {
		log.Trace(resp.StatusCode(), util.UnsafeBytesToString(util.EscapeNewLine(data)), util.UnsafeBytesToString(util.EscapeNewLine(resbody)))
	}

	if retryTimes>0{
		log.Errorf("#%v, code:%v",retryTimes,resp.StatusCode())
	}

	if resp.StatusCode() == http.StatusOK || resp.StatusCode() == http.StatusCreated {
		
		//如果是部分失败，应该将可以重试的做完，然后记录失败的消息再返回不继续
		if util.ContainStr(string(req.Header.RequestURI()), "_bulk") {

			containError, statsCodeStats := HandleBulkResponse2(tag, joint.Config.SafetyParse, data, resbody, joint.Config.DocBufferSize, successItems, nonRetryableItems, retryableItems,joint.Config.IncludeBusyRequestsToFailureQueue)

			for k,v:=range statsCodeStats{
				stats.IncrementBy("bulk::"+tag,util.ToString(k), int64(v))
			}

			if retryTimes>0{
				log.Errorf("#%v, code:%v, contain_err:%v, status:%v",retryTimes,resp.StatusCode(),containError,statsCodeStats)
			}

			if containError {
				count:=retryableItems.GetMessageCount()
				if count > 0 {
					log.Debugf("%v, retry item: %v",tag,count)
					bodyBytes:=retryableItems.GetMessageBytes()
					if !util.IsBytesEndingWith(&bodyBytes,NEWLINEBYTES){
						if !util.BytesHasPrefix(retryableItems.GetMessageBytes(),NEWLINEBYTES){
							retryableItems.WriteByteBuffer(NEWLINEBYTES)
							bodyBytes=retryableItems.GetMessageBytes()
						}
					}

					req.SetRawBody(bodyBytes)
					delayTime := joint.Config.RejectDelayInSeconds

					if delayTime <= 0 {
						delayTime = 5
					}
					
					time.Sleep(time.Duration(delayTime) * time.Second)
					
					if joint.Config.MaxRejectRetryTimes <= 0 {
						joint.Config.MaxRejectRetryTimes = 12 //1min
					}
					
					if retryTimes >= joint.Config.MaxRejectRetryTimes {
						
						//continue retry before is back
						if !metadata.IsAvailable() {
							return false, errors.Errorf("elasticsearch [%v] is not available", metadata.Config.Name)
						}
						
						data := req.OverrideBodyEncode(bodyBytes, true)
						queue.Push(queue.GetOrInitConfig(metadata.Config.ID+"_dead_letter_queue"), data)
						return true, errors.Errorf("bulk partial failure, retried %v times, quit retry", retryTimes)
					}
					log.Infof("%v, bulk partial failure, #%v retry, %v items left, size: %v", tag,retryTimes,retryableItems.GetMessageCount(),retryableItems.GetMessageSize())
					retryTimes++
					stats.Increment("elasticsearch."+tag+"."+metadata.Config.Name+".bulk", "retry")
					goto DO
				}

				//TODO, save message offset and failure message
				//if len(failureStatus) > 0 {
				//	failureStatusStr := util.JoinMapInt(failureStatus, ":")
				//	log.Debugf("documents in failure: %v", failureStatusStr)
				//	//save message bytes, with metadata, set codec to wrapped bulk messages
				//	queue.Push(queue.GetOrInitConfig("failure_messages"), util.MustToJSONBytes(util.MapStr{
				//		"cluster_id": metadata.Config.ID,
				//		"queue":      buffer.Queue,
				//		"request": util.MapStr{
				//			"uri":  req.URI().String(),
				//			"body": util.SubString(util.UnsafeBytesToString(req.GetRawBody()), 0, 1024*4),
				//		},
				//		"response": util.MapStr{
				//			"status": failureStatusStr,
				//			"body":   util.SubString(util.UnsafeBytesToString(resbody), 0, 1024*4),
				//		},
				//	}))
				//	//log.Errorf("bulk requests failure,host:%v,status:%v,invalid:%v,failure:%v,res:%v", host, statsCodeStats, nonRetryableItems.GetMessageCount(), retryableItems.GetMessageCount(), util.SubString(util.UnsafeBytesToString(resbody), 0, 1024))
				//}
				
				//skip all failure messages
				if nonRetryableItems.GetMessageCount() > 0 && retryableItems.GetMessageCount() == 0 {
					////handle 400 error
					if joint.Config.InvalidRequestsQueue != "" {
						queue.Push(queue.GetOrInitConfig(joint.Config.InvalidRequestsQueue), data)
					}
					return true, errors.Errorf("[%v] invalid bulk requests", metadata.Config.Name)
				}
				return false, errors.Errorf("bulk response contains error, %v", statsCodeStats)
			}
		}
		return true, nil
	} else if resp.StatusCode() == 429 {
		//TODO, save message offset and failure message
		time.Sleep(2 * time.Second)
		return false, errors.Errorf("code 429, [%v] is too busy", metadata.Config.Name)
	} else if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
		//TODO, save message offset and failure message
		////handle 400 error
		if joint.Config.InvalidRequestsQueue != "" {
			queue.Push(queue.GetOrInitConfig(joint.Config.InvalidRequestsQueue), data)
		}
		return true, errors.Errorf("invalid requests, code: %v", resp.StatusCode())
	} else {

		//TODO, save message offset and failure message
		//if joint.QueueConfig.SaveFailure {
		//	queue.Push(queue.GetOrInitConfig(joint.QueueConfig.FailureRequestsQueue), data)
		//}
		if global.Env().IsDebug {
			log.Debugf("status:", resp.StatusCode(), ",request:", util.UnsafeBytesToString(req.GetRawBody()), ",response:", util.UnsafeBytesToString(resp.GetRawBody()))
		}
		return false, errors.Errorf("bulk requests failed, code: %v", resp.StatusCode())
	}
}

func HandleBulkResponse2(tag string, safetyParse bool, requestBytes, resbody []byte, docBuffSize int, successItems *BulkBuffer, nonRetryableItems, retryableItems *BulkBuffer,retry429 bool) (bool, map[int]int) {
	nonRetryableItems.Reset()
	retryableItems.Reset()
	successItems.Reset()

	containError := util.LimitedBytesSearch(resbody, []byte("\"errors\":true"), 64)
	var statsCodeStats = map[int]int{}
	//decode response
	response := BulkResponse{}
	err := util.FromJSONBytes(resbody,&response)
	if err != nil {
		panic(err)
	}
	invalidOffset := map[int]BulkActionMetadata{}
	var validCount = 0
	for i, v := range response.Items {
		item := v.GetItem()

		x, ok := statsCodeStats[item.Status]
		if !ok {
			x = 0
		}
		x++
		statsCodeStats[item.Status] = x

		if item.Error != nil {
			invalidOffset[i] = v
		} else {
			validCount++
		}
	}

	if len(invalidOffset) > 0 {
		if global.Env().IsDebug {
			log.Debug(tag," bulk invalid, status:", statsCodeStats)
		}
	}
	var offset = 0
	var match = false
	var retryable = false
	var actionMetadata BulkActionMetadata
	var docBuffer []byte
	docBuffer = BulkDocBuffer.Get(docBuffSize)
	defer BulkDocBuffer.Put(docBuffer)

	WalkBulkRequests(safetyParse, requestBytes, docBuffer, func(eachLine []byte) (skipNextLine bool) {
		return false
	}, func(metaBytes []byte, actionStr, index, typeName, id,routing string) (err error) {
		actionMetadata, match = invalidOffset[offset]
		item:=actionMetadata.GetItem()

		if match {
			if item.Status==429 && retry429{
				retryable = true
			}else if item.Status >= 400 && item.Status < 500{ //find invalid request 409
				retryable = false
			} else {
				retryable = true
			}

			if retryable{
				retryableItems.WriteNewByteBufferLine("meta4",metaBytes)
				retryableItems.WriteMessageID(item.ID)
			}else{
				nonRetryableItems.WriteNewByteBufferLine("meta3",metaBytes)
				nonRetryableItems.WriteMessageID(item.ID)
			}
		}else{
			//fmt.Println(successItems!=nil,item!=nil,offset,string(metaBytes),id)
			successItems.WriteNewByteBufferLine("meta5",metaBytes)
			successItems.WriteMessageID(id)
		}
		offset++
		return nil
	}, func(payloadBytes []byte) {
		if match {
			if payloadBytes != nil && len(payloadBytes) > 0 {
				if retryable {
					retryableItems.WriteNewByteBufferLine("payload4",payloadBytes)
				} else {
					nonRetryableItems.WriteNewByteBufferLine("payload3",payloadBytes)
				}
			}
		}else{
			if payloadBytes != nil && len(payloadBytes) > 0 {
				successItems.WriteNewByteBufferLine("payload5", payloadBytes)
			}
		}
	})

	return containError, statsCodeStats
}
