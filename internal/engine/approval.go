package engine

import (
    "github.com/nuknal/PocketFlowGo/internal/store"
)

func (e *Engine) runApproval(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
    rt, _ := shared["_rt"].(map[string]interface{})
    if rt == nil { rt = map[string]interface{}{} }
    key := "ap:" + curr
    ap, _ := rt[key].(map[string]interface{})
    if ap == nil { ap = map[string]interface{}{} }
    approvalKey := ""
    if v, ok := params["approval_key"].(string); ok { approvalKey = v }
    val := resolveRef(approvalKey, shared, params, input)
    decided := false
    action := node.Post.ActionStatic
    if val != nil && val != "" {
        decided = true
        if node.Post.ActionKey != "" {
            action = pickAction(map[string]interface{}{"approval": val}, node.Post.ActionKey)
        } else {
            switch vv := val.(type) {
            case bool:
                if vv { action = "approved" } else { action = "rejected" }
            case string:
                if vv != "" { action = vv }
            }
        }
    }
    if decided {
        if node.Post.OutputKey != "" { shared[node.Post.OutputKey] = val }
        delete(rt, key)
        if len(rt) == 0 { delete(shared, "_rt") } else { shared["_rt"] = rt }
        e.recordRun(t, curr, 1, "ok", map[string]interface{}{"approval_key": approvalKey}, input, val, "", action, "", "")
        return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
    }
    rt[key] = ap
    shared["_rt"] = rt
    _ = e.Store.UpdateTaskStatus(t.ID, "running")
    _ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1)
    return nil
}

