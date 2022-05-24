package scheduling

type Scheduler interface {
	Schedule(deployment Deployment) error
	Unschedule(selector Selector) error
	GetDeployment(selector Selector) (Deployment, error)
	ListDeployments() ([]Deployment, error)
}

type Selector struct {
	Name   string
	Module string
}

type Deployment struct {
	Selector
	Image            string
	Volumes          []string
	Env              map[string]string
	Clusterable      bool
	ExposeExternally bool
	Ports            []uint16
}

func (selector Selector) GetIdentifier(schedulerName string) string {
	return schedulerName + "-" + selector.Module + "-" + selector.Name
}

type Volume struct {
	Selector
}
