package engine

import (
	"github.com/nuknal/PocketFlowGo/internal/store"
)

// runChoice executes a node of kind 'choice'.
// It evaluates conditions to determine the next path in the flow.
func (e *Engine) runChoice(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	action := ""

	// Evaluate choice cases in order
	if len(node.ChoiceCases) > 0 {
		for _, cc := range node.ChoiceCases {
			if evalExpr(cc.Expr, shared, params, input) {
				action = cc.Action
				break
			}
		}
	}

	// Fallback to other action determination methods if no case matched
	if action == "" && node.Post.ActionStatic != "" {
		action = node.Post.ActionStatic
	} else if action == "" && node.Post.ActionKey != "" {
		if input != nil {
			action = pickAction(map[string]interface{}{"v": input}, node.Post.ActionKey)
		}
		if action == "" {
			action = pickAction(shared, node.Post.ActionKey)
		}
		if action == "" && node.DefaultAction != "" {
			action = node.DefaultAction
		}
	} else if action == "" && node.DefaultAction != "" {
		action = node.DefaultAction
	}

	// Store output if configured
	if node.Post.OutputKey != "" {
		shared[node.Post.OutputKey] = input
	}

	e.logf("task=%s node=%s kind=choice action=%s", t.ID, curr, action)
	e.recordRun(t, curr, 1, "ok", map[string]interface{}{"input_key": node.Prep.InputKey}, input, nil, "", action, "", "")
	return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
}
