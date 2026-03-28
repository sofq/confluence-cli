package template

// builtinTemplates contains the default content templates shipped with cf.
// Bodies use Confluence storage format (XHTML) with {{.variable}} placeholders.
var builtinTemplates = map[string]*Template{
	"blank": {
		Title: "{{.title}}",
		Body:  "",
	},
	"meeting-notes": {
		Title: "{{.title}}",
		Body:  `<h2>Attendees</h2><p>{{.attendees}}</p><h2>Agenda</h2><p>{{.agenda}}</p><h2>Notes</h2><p></p><h2>Action Items</h2><ul><li></li></ul>`,
	},
	"decision": {
		Title: "{{.title}}",
		Body:  `<h2>Status</h2><p>{{.status}}</p><h2>Context</h2><p>{{.context}}</p><h2>Decision</h2><p>{{.decision}}</p><h2>Consequences</h2><p>{{.consequences}}</p>`,
	},
	"runbook": {
		Title: "{{.title}}",
		Body:  `<h2>Overview</h2><p>{{.overview}}</p><h2>Prerequisites</h2><ul><li>{{.prerequisites}}</li></ul><h2>Steps</h2><ol><li>{{.steps}}</li></ol><h2>Rollback</h2><p>{{.rollback}}</p>`,
	},
	"retrospective": {
		Title: "{{.title}}",
		Body:  `<h2>What Went Well</h2><p>{{.went_well}}</p><h2>What Could Be Improved</h2><p>{{.improvements}}</p><h2>Action Items</h2><ul><li>{{.actions}}</li></ul>`,
	},
	"adr": {
		Title: "{{.title}}",
		Body:  `<h2>Status</h2><p>{{.status}}</p><h2>Context</h2><p>{{.context}}</p><h2>Decision</h2><p>{{.decision}}</p><h2>Consequences</h2><p>{{.consequences}}</p><h2>Alternatives Considered</h2><p>{{.alternatives}}</p>`,
	},
}
