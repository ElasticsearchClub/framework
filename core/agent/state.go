/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

var stateManager IStateManager

func GetStateManager() IStateManager {
	if stateManager == nil {
		panic("agent state manager not init")
	}
	return stateManager
}

func RegisterStateManager(sm IStateManager) {
	stateManager = sm
}

func IsEnabled() bool {
	return stateManager != nil
}

type IStateManager interface {
	GetAgent(ID string) (*Instance, error)
	UpdateAgent(inst *Instance, syncToES bool) (*Instance, error)
	DispatchAgent(clusterID string) (*Instance, error)
	GetTaskAgent(clusterID string) (*Instance, error)
	SetAgentTask(clusterID, agentID string, nodeUUID string) error
	StopAgentTask(clusterID, agentID string, nodeUUID string) error
	DeleteAgent(agentID string) error
	LoopState()
	Stop()
	GetState(clusterID string) ShortState
	EnrollAgent(inst *Instance, confirmInfo interface{}) error
	DispatchNodeMetricTask(clusterID string) error
}
