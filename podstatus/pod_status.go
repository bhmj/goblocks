package podstatus

type PodStatus interface {
	IsReady() bool
	IsAlive() bool
}
