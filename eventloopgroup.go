package cnet

type IEventTcpLoopGroup interface {
	register(loop *eventTcpLoop)
	next() *eventTcpLoop
	iterate(func(*eventTcpLoop) bool)
	len() int
}

type roundRobinEventLoopGroup struct {
	nextLoopIndex int
	eventLoops    []*eventTcpLoop
	size          int
}

func (g *roundRobinEventLoopGroup) register(el *eventTcpLoop) {
	g.eventLoops = append(g.eventLoops, el)
	g.size++
}

func (g *roundRobinEventLoopGroup) next() (el *eventTcpLoop) {
	el = g.eventLoops[g.nextLoopIndex]
	if g.nextLoopIndex++; g.nextLoopIndex >= g.size {
		g.nextLoopIndex = 0
	}
	return
}

func (g *roundRobinEventLoopGroup) iterate(f func(*eventTcpLoop) bool) {
	for _, el := range g.eventLoops {
		if !f(el) {
			break
		}
	}
}

func (g *roundRobinEventLoopGroup) len() int {
	return g.size
}
