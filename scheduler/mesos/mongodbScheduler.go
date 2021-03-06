package mesos

import (
	//"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/gogo/protobuf/proto"
	mesos "github.com/mesos/mesos-go/mesosproto"
	sched "github.com/mesos/mesos-go/scheduler"
	//util "github.com/mesos/mesos-go/mesosutil"
	//"time"
	//"encoding/json"
	//"github.com/tangmingdong123/mongodb-mesos/scheduler/repo"
	"strings"
)

type MongodbScheduler struct {
}

func Start(master *string) {
	log.Debugln("startScheduler master:", *master)

	fwinfo := &mesos.FrameworkInfo{
		User: proto.String(""),
		Name: proto.String("mongodb-mesos"),
		FailoverTimeout: proto.Float64(24*3600*1000),
		Checkpoint: proto.Bool(true),
	}

	config := sched.DriverConfig{
		Scheduler: newMongodbScheduler(),
		Framework: fwinfo,
		Master:    *master,
	}

	driver, err := sched.NewMesosSchedulerDriver(config)
	if err != nil {
		log.Errorln("Unable to create a SchedulerDriver ", err.Error())
	}

	stat, err := driver.Run()
	if err != nil {
		log.Infof("Framework stopped with status %s and error: %s", stat.String(), err.Error())
	}

	log.Infof("stat:%v", stat)
}

func newMongodbScheduler() *MongodbScheduler {
	return &MongodbScheduler{}
}

func (sched *MongodbScheduler) Registered(driver sched.SchedulerDriver, frameworkId *mesos.FrameworkID, masterInfo *mesos.MasterInfo) {
	log.Infoln("Framework Registered with Master ", masterInfo)
}

func (sched *MongodbScheduler) Reregistered(driver sched.SchedulerDriver, masterInfo *mesos.MasterInfo) {
	log.Infoln("Framework Re-Registered with Master ", masterInfo)
}

func (sched *MongodbScheduler) Disconnected(sched.SchedulerDriver) {
	log.Warningf("disconnected from master")
}

func (sched *MongodbScheduler) ResourceOffers(driver sched.SchedulerDriver, offers []*mesos.Offer) {
	//log.Warningf("Framework resourceOffer")

	/*
		for _, offer := range offers {
			bytes, _ := json.Marshal(offer)
			log.Infof("offer:%s", string(bytes))
		}
	*/

	var idleIDs []*mesos.OfferID
	var usedIDs []*mesos.OfferID
	usedMap := make(map[*mesos.Offer]*Used)

	//handle standalone first
	handleStandalone(driver, offers, idleIDs, usedIDs, usedMap)

	//handle replica second
	handleReplicaSet(driver, offers, idleIDs, usedIDs, usedMap)

	//unused offer
	for _, offer := range offers {
		used := false
		for _, usedid := range usedIDs {
			if offer.GetId() == usedid {
				used = true
				break
			}
		}
		if !used {
			idleIDs = append(idleIDs, offer.GetId())
		}
	}
	//reject offer
	driver.LaunchTasks(idleIDs, make([]*mesos.TaskInfo, 0), &mesos.Filters{RefuseSeconds: proto.Float64(5)})
}

func (sched *MongodbScheduler) StatusUpdate(driver sched.SchedulerDriver, status *mesos.TaskStatus) {
	log.Infoln("Status update: task", status.TaskId.GetValue(), " is in state ", status.State.Enum().String())
	log.Infof("reason:%v,message:%v,source:%v\n", status.GetReason().Enum(), status.GetMessage(), status.GetSource())

	//bs, _ := json.Marshal(status)
	//log.Infof("Status info %v", string(bs))

	if strings.Contains(status.GetTaskId().GetValue(), PREFIX_TASK_STANDALONE) {
		updateStandaloneStatus(status)
	} else if strings.Contains(status.GetTaskId().GetValue(), PREFIX_TASK_REPLICA) {
		updateReplicaStatus(status)
	} else if strings.Contains(status.GetTaskId().GetValue(), PREFIX_TASK_REPLICA_INIT) {
		updateReplicaInitStatus(status)
	}

}

func (sched *MongodbScheduler) OfferRescinded(_ sched.SchedulerDriver, oid *mesos.OfferID) {
	log.Errorf("offer rescinded: %v", oid)
}
func (sched *MongodbScheduler) FrameworkMessage(_ sched.SchedulerDriver, eid *mesos.ExecutorID, sid *mesos.SlaveID, msg string) {
	log.Errorf("framework message from executor %q slave %q: %q", eid, sid, msg)
}
func (sched *MongodbScheduler) SlaveLost(_ sched.SchedulerDriver, sid *mesos.SlaveID) {
	log.Errorf("slave lost: %v", sid)
}
func (sched *MongodbScheduler) ExecutorLost(_ sched.SchedulerDriver, eid *mesos.ExecutorID, sid *mesos.SlaveID, code int) {
	log.Errorf("executor %q lost on slave %q code %d", eid, sid, code)
}
func (sched *MongodbScheduler) Error(_ sched.SchedulerDriver, err string) {
	log.Errorf("Scheduler received error: %v", err)
}
