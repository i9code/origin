package cluster

import (
	"encoding/json"
	"fmt"
	"github.com/duanhf2012/origin/log"
	"io/ioutil"
	"strings"
)

func (slf *Cluster) ReadClusterConfig(filepath string) (*SubNet,error) {
	c := &SubNet{}
	d, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(d, c)
	if err != nil {
		return nil, err
	}

	return c,nil
}


func (slf *Cluster) ReadServiceConfig(filepath string)  (map[string]interface{},map[int]map[string]interface{},error) {

	c := map[string]interface{}{}

	d, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil,nil, err
	}
	err = json.Unmarshal(d, &c)
	if err != nil {
		return nil,nil, err
	}

	serviceConfig := map[string]interface{}{}
	serviceCfg,ok  := c["Service"]
	if ok == true {
		serviceConfig = serviceCfg.(map[string]interface{})
	}

	mapNodeService := map[int]map[string]interface{}{}
	nodeServiceCfg,ok  := c["NodeService"]
	if ok == true {
		nodeServiceList := nodeServiceCfg.([]interface{})
		for _,v := range nodeServiceList{
			serviceCfg :=v.(map[string]interface{})
			nodeid,ok := serviceCfg["NodeId"]
			if ok == false {
				log.Fatal("nodeservice list not find nodeid field: %+v",nodeServiceList)
			}
			mapNodeService[int(nodeid.(float64))] = serviceCfg
		}
	}
	return serviceConfig,mapNodeService,nil
}

func (slf *Cluster) ReadAllSubNetConfig() error {
	clusterCfgPath :=strings.TrimRight(configdir,"/")  +"/cluster"
	fileInfoList,err := ioutil.ReadDir(clusterCfgPath)
	if err != nil {
		return fmt.Errorf("Read dir %s is fail :%+v",clusterCfgPath,err)
	}

	slf.mapSubNetInfo =map[string] SubNet{}
	for _,f := range fileInfoList{
		if f.IsDir() == true {
			filePath := strings.TrimRight(strings.TrimRight(clusterCfgPath,"/"),"\\")+"/"+f.Name()+"/"+"cluster.json"
			subnetinfo,err:=slf.ReadClusterConfig(filePath)
			if err != nil {
				return fmt.Errorf("read file path %s is error:%+v" ,filePath,err)
			}
			slf.mapSubNetInfo[f.Name()] = *subnetinfo
		}
	}

	return nil
}

func (slf *Cluster) ReadLocalSubNetServiceConfig(subnet string) error {
	clusterCfgPath :=strings.TrimRight(configdir,"/")  +"/cluster"
	fileInfoList,err := ioutil.ReadDir(clusterCfgPath)
	if err != nil {
		return fmt.Errorf("Read %s dir is fail:%+v ",clusterCfgPath,err)
	}

	slf.mapSubNetInfo =map[string] SubNet{}
	for _,f := range fileInfoList{
		if f.IsDir() == true && f.Name()==subnet{ //????????????
			filePath := strings.TrimRight(strings.TrimRight(clusterCfgPath,"/"),"\\")+"/"+f.Name()+"/"+"service.json"
			localServiceCfg,localNodeServiceCfg,err:=slf.ReadServiceConfig(filePath)
			if err != nil {
				return fmt.Errorf("Read file %s is fail :%+v",filePath,err)
			}
			slf.localServiceCfg = localServiceCfg
			slf.localNodeServiceCfg =localNodeServiceCfg
		}
	}

	return nil
}



func (slf *Cluster) InitCfg(currentNodeId int) error{
	//mapSubNetInfo  := map[string] SubNet{} //???????????????????????????
	mapSubNetNodeInfo := map[string]map[int]NodeInfo{} //map[????????????]map[NodeId]NodeInfo
	localSubNetMapNode := map[int]NodeInfo{}           //???????????? map[NodeId]NodeInfo
	localSubNetMapService := map[string][]NodeInfo{}   //??????????????????ServiceName?????????????????????
	localNodeMapService := map[string]interface{}{}    //???Node???????????????
	localNodeInfo := NodeInfo{}

	err := slf.ReadAllSubNetConfig()
	if err != nil {
		return err
	}

	//????????????
	var localSubnetName string
	for subnetName,subnetInfo := range slf.mapSubNetInfo {
		for _,nodeinfo := range subnetInfo.NodeList {
			//??????slf.mapNodeInfo
			_,ok := mapSubNetNodeInfo[subnetName]
			if ok == false {
				mapnodeInfo := make(map[int]NodeInfo,1)
				mapnodeInfo[nodeinfo.NodeId] = nodeinfo
				mapSubNetNodeInfo[subnetName] = mapnodeInfo
			}else{
				mapSubNetNodeInfo[subnetName][nodeinfo.NodeId] = nodeinfo
			}

			//????????????????????????
			if nodeinfo.NodeId == currentNodeId {
				localSubnetName = subnetName
			}
		}
	}


	//??????
	subnet,ok := slf.mapSubNetInfo[localSubnetName]
	if ok == false {
		return fmt.Errorf("NodeId %d not in any subnet",currentNodeId)
	}
	subnet.SubNetName = localSubnetName
	for _,nodeinfo := range subnet.NodeList {
		localSubNetMapNode[nodeinfo.NodeId] = nodeinfo

		//?????????Node?????????????????????
		if nodeinfo.NodeId == currentNodeId {
			for _,s := range nodeinfo.ServiceList {
				servicename := s
				if strings.Index(s,"_") == 0 {
					servicename = s[1:]
				}
				localNodeMapService[servicename] = nil
			}
			localNodeInfo = nodeinfo
		}

		for _,s := range nodeinfo.ServiceList {
			//???_???????????????????????????????????????????????????????????????
			if strings.Index(s,"_") == 0 {
				continue
			}

			if _,ok := localSubNetMapService[s];ok== true{
				localSubNetMapService[s] = []NodeInfo{}
			}
			localSubNetMapService[s] = append(localSubNetMapService[s],nodeinfo)
		}
	}
	if localNodeInfo.NodeId == 0 {
		return fmt.Errorf("Canoot find NodeId %d not in any config file.",currentNodeId)
	}


	slf.mapSubNetNodeInfo=mapSubNetNodeInfo
	slf.localSubNetMapNode=localSubNetMapNode
	slf.localSubNetMapService = localSubNetMapService
	slf.localNodeMapService = localNodeMapService
	slf.localsubnet = subnet
	slf.localNodeInfo =localNodeInfo

	//????????????
	return slf.ReadLocalSubNetServiceConfig(slf.localsubnet.SubNetName)
}


func (slf *Cluster) IsConfigService(servicename string) bool {
	_,ok := slf.localNodeMapService[servicename]
	return ok
}

func (slf *Cluster) GetNodeIdByService(servicename string) []int{
	var nodelist []int
	nodeInfoList,ok := slf.localSubNetMapService[servicename]
	if ok == true {
		for _,node := range nodeInfoList {
			nodelist = append(nodelist,node.NodeId)
		}
	}

	return nodelist
}

func (slf *Cluster) getServiceCfg(servicename string) interface{}{
	v,ok := slf.localServiceCfg[servicename]
	if ok == false {
		return nil
	}

	return v
}

func (slf *Cluster) GetServiceCfg(nodeid int,servicename string) interface{}{
	nodeService,ok := slf.localNodeServiceCfg[nodeid]
	if ok == false {
		return slf.getServiceCfg(servicename)
	}

	v,ok := nodeService[servicename]
	if ok == false{
		return slf.getServiceCfg(servicename)
	}

	return v
}
