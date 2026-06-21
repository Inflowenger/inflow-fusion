package nats

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron"
)

var schedHander *gocron.Scheduler

func getSchedHandler() *gocron.Scheduler {
	if schedHander == nil {
		schedHander = gocron.NewScheduler(time.UTC)
		schedHander.StartAsync()
		if schedHander.IsRunning() {
			fmt.Println("hub connection ttl is running ...")
		}
	}
	return schedHander
}

func CleanConnCron(spaceId string) {
	getSchedHandler().RemoveByTagsAny(spaceId)
	finded, err := getSchedHandler().FindJobsByTag(spaceId)
	if err == nil {
		for _, job := range finded {
			fmt.Println("Account in cron : ", job.GetName())
			getSchedHandler().RemoveByReference(job)
		}
	}
	fmt.Printf("Number Of hub Accounts Connections: %d\n", len(getSchedHandler().Jobs()))

}
func ReleaseHubConnection(spaceId string) {

	if GetNatsBox() != nil {
		_, ok := GetNatsBox().Read(spaceId)
		if ok {
			refreshSched(spaceId)
		}

	}

}


func refreshSched(spaceId string) {

	getSchedHandler().RemoveByTagsAny(spaceId)
	finded, err := getSchedHandler().FindJobsByTag(spaceId)
	if err == nil {
		for _, job := range finded {
			getSchedHandler().RemoveByReference(job)
		}
	} else {
		fmt.Println("DEBUG LOG: NatsConnectionManager ", err.Error())
	}
	// 0 20 * * * *  = 20 minutes past every hour
	// * 20 * * * * = every minute at 20 seconds
	// 0 0/15 * * * * = every 15 minutes past the hour
	// 20 * * * * * * = every minute at 20 seconds
	// 0/20 * * * * * * = every 20 seconds
	getSchedHandler().Tag(spaceId).Name(spaceId).CronWithSeconds(fmt.Sprintf("%d */1 * * * *", time.Now().Second())).LimitRunsTo(1).Do(func(spaceId string) {
		if GetNatsBox() != nil {
			defer func() {
				fmt.Printf("Number Of hub Accounts GetNatsBox(): %d\n", len(getSchedHandler().Jobs()))
				fmt.Println("acc count con:", len(GetNatsBox().nats))
			}()
			n, ok := GetNatsBox().Read(spaceId)
			if ok {
				n.con.Drain()
				n.con.Close()
				GetNatsBox().Delete(spaceId)
				fmt.Println("Cron Clean By Key :", spaceId)
			}
		}
	}, spaceId)

	fmt.Printf("Number Of hub Accounts Connections: %d\n", len(getSchedHandler().Jobs()))
	fmt.Println("acc count con:", len(GetNatsBox().nats))

}
