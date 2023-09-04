/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"fmt"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"strconv"
	"strings"
	"time"
)

func Itoa64(i int64) string {
	return strconv.FormatInt(i, 10)
}

func AcquireOffset(seg, pos int64) Offset {
	return Offset{Segment: seg, Position: pos}
}

func ConvertOffset(offsetStr string) (int64, int64) {
	if offsetStr == "" {
		panic(errors.New("offset can't be empty"))
	}

	data := strings.Split(offsetStr, ",")
	if len(data) != 2 {
		panic(errors.Errorf("invalid offset: %v", offsetStr))
	}
	var segment, offset int64
	segment, _ = util.ToInt64(data[0])
	offset, _ = util.ToInt64(data[1])
	return segment, offset
}

func NewOffsetFromStr(offsetStr string) Offset {
	return NewOffset(ConvertOffset(offsetStr))
}
func NewOffset(seg, pos int64) Offset {
	return Offset{Segment: seg, Position: pos}
}

type Offset struct {
	Segment  int64 `json:"segment"`
	Position int64 `json:"position"`
}

func (c *Offset) Equals(v Offset) bool {
	if c.Segment != v.Segment {
		return false
	}

	if c.Position<=0 && v.Position<=0 {
		return true
	}

	if c.Position != v.Position {
		return false
	}

	return true
}

func (c *Offset) LatestThan(v Offset) bool {
	if c.Segment > v.Segment {
		return true
	}

	if c.Position > v.Position{
		return true
	}

	return false
}

func (c *Offset) String() string {
	return fmt.Sprintf("%v,%v", c.Segment, c.Position)
}

type Context struct {
	MessageCount int    `config:"message_count" json:"message_count"`
	NextOffset   Offset `config:"next_offset" json:"next_offset"`
	InitOffset   Offset `config:"init_offset" json:"init_offset"`
}

func (c *Context) UpdateInitOffset(seg, pos int64) {
	c.InitOffset.Segment = seg
	c.InitOffset.Position = pos
}

func (c *Context) UpdateNextOffset(seg, pos int64) {
	c.NextOffset.Segment = seg
	c.NextOffset.Position = pos
}

func (c *Context) String() string {
	return fmt.Sprintf("%v->%v", c.InitOffset, c.NextOffset)
}

func (c *Context) Valid() bool {
	return c.MessageCount > 0
}

type Message struct {
	Timestamp  int64  `config:"timestamp" json:"timestamp" parquet:"timestamp"`
	Offset     Offset `config:"offset" json:"offset"  parquet:"offset"`                //current offset
	NextOffset Offset `config:"next_offset" json:"next_offset"  parquet:"next_offset"` //offset for next message
	Size       int    `config:"size" json:"size"  parquet:"size"`
	Data       []byte `config:"data" json:"data"  parquet:"data,zstd"`
}

func (m *Message) String() string {
	return fmt.Sprintf("timestamp:%v, offset:%v, next_offset:%v, size:%v, data:%v", time.Unix(0, m.Timestamp), m.Offset.String(), m.NextOffset.String(), m.Size, string(m.Data))
}

type ProduceRequest struct {
	Topic string `config:"topic" json:"topic"` //queue_id
	Key   []byte `config:"key" json:"key"`
	Data  []byte `config:"data" json:"data"`
}

type ProduceResponse struct {
	Topic     string `config:"topic" json:"topic"`
	Partition int64  `config:"partition" json:"partition"`
	Offset    Offset `config:"offset" json:"offset"`
	Timestamp int64  `config:"timestamp" json:"timestamp"`
}
