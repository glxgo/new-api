package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestPublicErrorTaskProjection(t *testing.T) {
	task := &model.Task{Status: model.TaskStatusFailure, FailReason: "https://private.example/task failed", Data: []byte(`{"error":{"message":"https://private.example/task"},"detail":"https://private.example/debug"}`)}
	output, err := common.Marshal(TaskModel2Dto(task))
	require.NoError(t, err)
	require.NotContains(t, string(output), "private.example")
	require.Empty(t, TaskModel2Dto(task).ResultURL)
	require.Contains(t, task.FailReason, "private.example")
	task.Status = model.TaskStatusSuccess
	task.FailReason = "https://images.example/legacy.mp4"
	task.Data = []byte(`{"url":"https://images.example/legacy.mp4"}`)
	result := TaskModel2Dto(task)
	require.Equal(t, task.FailReason, result.ResultURL)
	require.Equal(t, task.FailReason, result.FailReason)
	require.Equal(t, task.Data, result.Data)
}
