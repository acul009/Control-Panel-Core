package main

import (
	"controlPanel/scheduling"
	"controlPanel/scheduling/docker"
	"fmt"
)

func main() {
	scheduler, _ := docker.CreateDockerScheduler("testScheduler")
	err := scheduler.Schedule(scheduling.Deployment{
		Selector: scheduling.Selector{
			Name:   "testName",
			Module: "testModule",
		},
		Image: "nginx",
	})
	if err != nil {
		fmt.Println(err)

	}

	deployments, _ := scheduler.ListDeployments()
	fmt.Printf("%+v", deployments)
}
