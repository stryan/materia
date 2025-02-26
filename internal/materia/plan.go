package materia

type Plan struct {
	Actions []Action
	Volumes []Volume
}

func NewPlan() *Plan {
	return &Plan{}
}

func (p *Plan) Add(a Action) {
	p.Actions = append(p.Actions, a)
}

func (p *Plan) Append(a []Action) {
	p.Actions = append(p.Actions, a...)
}

func (p *Plan) Merge(other *Plan) {
	p.Actions = append(p.Actions, other.Actions...)
	p.Volumes = append(p.Volumes, other.Volumes...)
}

func (p *Plan) Empty() bool {
	return len(p.Actions) == 0
}

func (p *Plan) Validate() error {
	return nil
}
