package pipeline

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
)

type PipelineConfigV2 struct {
	Name           string `config:"name" json:"name,omitempty"`
	Enabled        *bool  `config:"enabled" json:"enabled,omitempty"`
	AutoStart      bool   `config:"auto_start" json:"auto_start"`
	KeepRunning    bool   `config:"keep_running" json:"keep_running"`
	RetryDelayInMs int    `config:"retry_delay_in_ms" json:"retry_delay_in_ms"`
	Logging        struct {
		Enabled bool `config:"enabled" json:"enabled"`
	} `config:"logging" json:"logging"`
	Processors []*config.Config       `config:"processor" json:"-"`
	Labels     map[string]interface{} `config:"labels" json:"labels"`

	Transient bool `config:"-" json:"transient"`
}

func (this PipelineConfigV2) Equals(target PipelineConfigV2) bool {
	if this.Name != target.Name ||
		this.AutoStart != target.AutoStart ||
		this.KeepRunning != target.KeepRunning ||
		this.RetryDelayInMs != target.RetryDelayInMs ||
		this.Logging.Enabled != target.Logging.Enabled ||
		!this.ProcessorsEquals(target) {
		return false
	}
	return true
}

func (this PipelineConfigV2) ProcessorsEquals(target PipelineConfigV2) bool {
	if len(this.Processors) != len(target.Processors) {
		return false
	}
	var length = len(this.Processors)
	for i := 0; i < length; i++ {
		srcM := map[string]interface{}{}
		err := this.Processors[i].Unpack(srcM)
		if err != nil {
			log.Error(err)
		}
		dstM := map[string]interface{}{}
		err = target.Processors[i].Unpack(dstM)
		if err != nil {
			log.Error(err)
		}
		clog, err := util.DiffTwoObject(srcM, dstM)
		if err != nil {
			log.Error(err)
		}
		if len(clog) > 0 {
			return false
		}
	}
	return true
}
