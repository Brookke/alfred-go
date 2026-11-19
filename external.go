package alfred

// ExternalNode is a trigger that scripts and other workflows can call.
type ExternalNode struct {
	node[*ExternalNode]
	id  string
	url bool
}

// External runs the nodes connected with Then when called with AppleScript:
//
//	tell application id "com.runningwithcrayons.Alfred" to run trigger "id" in workflow "com.example.bundle" with argument "query"
func External(id string) *ExternalNode {
	e := &ExternalNode{id: id}
	e.self, e.label = e, id
	return e
}

// URLHandler also lets alfred://runtrigger/<bundle id>/<id>/?argument=... URLs call the trigger.
func (e *ExternalNode) URLHandler() *ExternalNode {
	e.url = true
	return e
}

func (e *ExternalNode) object(string) (string, int, map[string]any) {
	return "alfred.workflow.trigger.external", 1, map[string]any{
		"triggerid":              e.id,
		"availableviaurlhandler": e.url,
	}
}
