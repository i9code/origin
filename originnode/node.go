package originnode

import (
	"fmt"
	"log"

	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/duanhf2012/origin/cluster"
	"github.com/duanhf2012/origin/service"
	"github.com/duanhf2012/origin/sysmodule"
)

type CExitCtl struct {
	exit      chan bool
	waitGroup *sync.WaitGroup
}

type COriginNode struct {
	CExitCtl
	serviceManager service.IServiceManager
	sigs           chan os.Signal
	debugCheck     bool
}

func (s *COriginNode) Init() {

	//初始化全局模块
	InitGlobalModule()
	service.InitLog()
	service.InstanceServiceMgr().Init()

	s.exit = make(chan bool)
	s.waitGroup = &sync.WaitGroup{}
	s.sigs = make(chan os.Signal, 1)
	signal.Notify(s.sigs, syscall.SIGINT, syscall.SIGTERM)
}

func (s *COriginNode) OpenDebugCheck() {
	s.debugCheck = true
}

func (s *COriginNode) SetupService(services ...service.IService) {
	for i := 0; i < len(services); i++ {
		if cluster.InstanceClusterMgr().HasLocalService(services[i].GetServiceName()) == true {
			service.InstanceServiceMgr().Setup(services[i])
		}

	}

	//将其他服务通知已经安装
	for i := 0; i < len(services); i++ {
		for j := 0; j < len(services); j++ {
			if cluster.InstanceClusterMgr().HasLocalService(services[i].GetServiceName()) == false {
				continue
			}

			if services[i].GetServiceName() == services[j].GetServiceName() {
				continue
			}

			services[i].OnSetupService(services[j])
		}
	}

}

func (s *COriginNode) Start() {
	if s.debugCheck == true {
		go func() {

			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	cluster.InstanceClusterMgr().Start()
	RunGlobalModule(s.exit, s.waitGroup)
	service.InstanceServiceMgr().Start(s.exit, s.waitGroup)

	select {
	case <-s.sigs:
		fmt.Println("收到信号推出程序")
	}

	s.Stop()
}

func (s *COriginNode) Stop() {
	close(s.exit)
	s.waitGroup.Wait()
}

func NewOrginNode() *COriginNode {
	var syslogmodule sysmodule.LogModule
	syslogmodule.Init("system", sysmodule.LEVER_INFO)
	syslogmodule.SetModuleType(sysmodule.SYS_LOG)
	AddModule(&syslogmodule)

	err := cluster.InstanceClusterMgr().Init()
	if err != nil {
		fmt.Print(err)
		return nil
	}

	return new(COriginNode)
}

func HasCmdParam(param string) bool {
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == param {
			return true
		}
	}

	return false
}
