package engine

// runChoice executes a node of kind 'choice'.
// It evaluates conditions to determine the next path in the flow.
func (e *Engine) runChoice(in NodeRunInput) error {
	action := ""

	// Evaluate choice cases in order
	if len(in.Node.ChoiceCases) > 0 {
		for _, cc := range in.Node.ChoiceCases {
			if evalExpr(cc.Expr, in.Shared, in.Params, in.Input) {
				action = cc.Action
				break
			}
		}
	}

	// Fallback to other action determination methods if no case matched
	if action == "" && in.Node.Post.ActionStatic != "" {
		action = in.Node.Post.ActionStatic
	} else if action == "" && in.Node.Post.ActionKey != "" {
		if in.Input != nil {
			action = pickAction(map[string]interface{}{"v": in.Input}, in.Node.Post.ActionKey)
		}
		if action == "" {
			action = pickAction(in.Shared, in.Node.Post.ActionKey)
		}
		if action == "" && in.Node.DefaultAction != "" {
			action = in.Node.DefaultAction
		}
	} else if action == "" && in.Node.DefaultAction != "" {
		action = in.Node.DefaultAction
	}

	// Store output if configured
	if in.Node.Post.OutputKey != "" {
		in.Shared[in.Node.Post.OutputKey] = in.Input
	}

	e.logf("task=%s node=%s kind=choice action=%s", in.Task.ID, in.NodeKey, action)
	e.recordRun(in.Task, in.NodeKey, 1, "ok", map[string]interface{}{"input_key": in.Node.Prep.InputKey}, in.Input, nil, "", action, "", "", "")
	return e.finishNode(in.Task, in.FlowDef, in.NodeKey, action, in.Shared, in.Task.StepCount+1, nil)
}
