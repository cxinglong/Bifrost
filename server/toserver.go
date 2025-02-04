package server

import (
	"sync"
	"log"
	"github.com/brokercap/Bifrost/plugin/driver"
	pluginStorage "github.com/brokercap/Bifrost/plugin/storage"

	"encoding/json"
)

type ToServer struct {
	sync.RWMutex
	ToServerID	  		int
	PluginName    		string
	MustBeSuccess 		bool
	FilterQuery			bool
	FilterUpdate		bool
	FieldList     		[]string
	ToServerKey   		string
	BinlogFileNum 		int
	BinlogPosition 		uint32
	PluginParam   		map[string]interface{}
	Status        		string
	ToServerChan  		*ToServerChan `json:"-"`
	Error		  		string
	ErrorWaitDeal 		int
	ErrorWaitData 		interface{}
	PluginConn	  		driver.ConnFun `json:"-"`
	PluginConnKey 		string `json:"-"`
	PluginParamObj 		interface{} `json:"-"`
	LastBinlogFileNum   int                       // 由 channel 提交到 ToServerChan 的最后一个位点
	LastBinlogPosition  uint32                    // 假如 BinlogFileNum == LastBinlogFileNum && BinlogPosition == LastBinlogPosition 则说明这个位点是没有问题的
	LastBinlogKey 		[]byte `json:"-"`         // 将数据保存到 level 的key
}

func (db *db) AddTableToServer(schemaName string, tableName string, toserver *ToServer) (bool,int) {
	key := GetSchemaAndTableJoin(schemaName,tableName)
	if _, ok := db.tableMap[key]; !ok {
		return false,0
	} else {
		db.Lock()
		if toserver.ToServerID <= 0{
			db.tableMap[key].LastToServerID += 1
			toserver.ToServerID = db.tableMap[key].LastToServerID
		}
		if toserver.PluginName == ""{
			ToServerInfo := pluginStorage.GetToServerInfo(toserver.ToServerKey)
			if ToServerInfo != nil{
				toserver.PluginName = ToServerInfo.PluginName
			}
		}
		if toserver.BinlogFileNum == 0{
			BinlogPostion,err := getBinlogPosition(getDBBinlogkey(db))
			if err == nil {
				toserver.BinlogFileNum = BinlogPostion.BinlogFileNum
				toserver.LastBinlogFileNum = BinlogPostion.BinlogFileNum
				toserver.BinlogPosition = BinlogPostion.BinlogPosition
				toserver.LastBinlogPosition = BinlogPostion.BinlogPosition
			}else{
				log.Println("AddTableToServer GetDBBinlogPostion:",err)
			}
		}
		db.tableMap[key].ToServerList = append(db.tableMap[key].ToServerList, toserver)
		db.Unlock()
		log.Println("AddTableToServer",db.Name,schemaName,tableName,toserver)
	}
	return true,toserver.ToServerID
}

func (db *db) DelTableToServer(schemaName string, tableName string,ToServerID int) bool {
	key := GetSchemaAndTableJoin(schemaName,tableName)
	if _, ok := db.tableMap[key]; !ok {
		return false
	} else {
		var index int = -1
		db.Lock()
		for index1,toServerInfo2 := range db.tableMap[key].ToServerList{
			if toServerInfo2.ToServerID == ToServerID{
				index = index1
				break
			}
		}
		if index == -1 {
			db.Unlock()
			return true
		}
		toServerInfo := db.tableMap[key].ToServerList[index]
		toServerPositionBinlogKey := getToServerBinlogkey(db,toServerInfo)
		//toServerInfo.Lock()
		db.tableMap[key].ToServerList = append(db.tableMap[key].ToServerList[:index], db.tableMap[key].ToServerList[index+1:]...)
		if toServerInfo.Status == "running"{
			toServerInfo.Status = "deling"
		}else{
			if toServerInfo.Status != "deling" {
				delBinlogPosition(toServerPositionBinlogKey)
			}
		}
		log.Println("DelTableToServer",db.Name,schemaName,tableName,"toServerInfo:",toServerInfo)
		db.Unlock()
	}
	return true
}

func (This *ToServer) AddWaitError(WaitErr error,WaitData interface{}) bool {
	This.Lock()
	This.Error = WaitErr.Error()
	b,_:=json.Marshal(WaitData)
	This.ErrorWaitData = string(b)
	This.Unlock()
	return true
}

func (This *ToServer) DealWaitError() bool {
	This.Lock()
	This.ErrorWaitDeal = 1
	This.Unlock()
	return true
}

func (This *ToServer) GetWaitErrorDeal() int {
	This.Lock()
	deal := This.ErrorWaitDeal
	This.Unlock()
	return deal
}

func (This *ToServer) DelWaitError() bool {
	This.Lock()
	This.Error = ""
	This.ErrorWaitData = nil
	This.ErrorWaitDeal = 0
	This.Unlock()
	return true
}