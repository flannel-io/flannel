package ecs

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/denverdino/aliyungo/common"
)

func ExampleClient_DescribeInstanceStatus() {
	fmt.Printf("DescribeInstanceStatus Example\n")

	args := DescribeInstanceStatusArgs{
		RegionId: "cn-beijing",
		ZoneId:   "cn-beijing-b",
		Pagination: common.Pagination{
			PageNumber: 1,
			PageSize:   1,
		},
	}

	client := NewTestClient()
	instanceStatus, _, err := client.DescribeInstanceStatus(&args)

	if err != nil {
		fmt.Printf("Failed to describe Instance: %s status:%v \n", TestInstanceId, err)
	} else {
		for i := 0; i < len(instanceStatus); i++ {
			fmt.Printf("Instance %s Status: %s \n", instanceStatus[i].InstanceId, instanceStatus[i].Status)
		}
	}
}

func ExampleClient_DescribeInstanceAttribute() {
	fmt.Printf("DescribeInstanceAttribute Example\n")

	client := NewTestClient()

	instanceAttributeType, err := client.DescribeInstanceAttribute(TestInstanceId)

	if err != nil {
		fmt.Printf("Failed to describe Instance %s attribute: %v\n", TestInstanceId, err)
	} else {
		fmt.Printf("Instance Information\n")
		fmt.Printf("InstanceId = %s \n", instanceAttributeType.InstanceId)
		fmt.Printf("InstanceName = %s \n", instanceAttributeType.InstanceName)
		fmt.Printf("HostName = %s \n", instanceAttributeType.HostName)
		fmt.Printf("ZoneId = %s \n", instanceAttributeType.ZoneId)
		fmt.Printf("RegionId = %s \n", instanceAttributeType.RegionId)
	}
}

func ExampleClient_DescribeInstanceVncUrl() {
	fmt.Printf("DescribeInstanceVncUrl Example\n")

	args := DescribeInstanceVncUrlArgs{
		RegionId:   "cn-beijing",
		InstanceId: TestInstanceId,
	}

	client := NewTestClient()

	instanceVncUrl, err := client.DescribeInstanceVncUrl(&args)

	if err != nil {
		fmt.Printf("Failed to describe Instance %s vnc url: %v \n", TestInstanceId, err)
	} else {
		fmt.Printf("VNC URL = %s \n", instanceVncUrl)
	}
}

func ExampleClient_StopInstance() {
	fmt.Printf("Stop Instance Example\n")

	client := NewTestClient()

	err := client.StopInstance(TestInstanceId, true)

	if err != nil {
		fmt.Printf("Failed to stop Instance %s vnc url: %v \n", TestInstanceId, err)
	}
}

func ExampleClient_DeleteInstance() {
	fmt.Printf("Delete Instance Example")

	client := NewTestClient()

	err := client.DeleteInstance(TestInstanceId)

	if err != nil {
		fmt.Printf("Failed to delete Instance %s vnc url: %v \n", TestInstanceId, err)
	}
}

func TestECSInstance(t *testing.T) {
	if TestQuick {
		return
	}
	client := NewTestClient()
	instance, err := client.DescribeInstanceAttribute(TestInstanceId)
	if err != nil {
		t.Fatalf("Failed to describe instance %s: %v", TestInstanceId, err)
	}
	t.Logf("Instance: %++v  %v", instance, err)
	err = client.StopInstance(TestInstanceId, true)
	if err != nil {
		t.Errorf("Failed to stop instance %s: %v", TestInstanceId, err)
	}
	err = client.WaitForInstance(TestInstanceId, Stopped, 0)
	if err != nil {
		t.Errorf("Instance %s is failed to stop: %v", TestInstanceId, err)
	}
	t.Logf("Instance %s is stopped successfully.", TestInstanceId)
	err = client.StartInstance(TestInstanceId)
	if err != nil {
		t.Errorf("Failed to start instance %s: %v", TestInstanceId, err)
	}
	err = client.WaitForInstance(TestInstanceId, Running, 0)
	if err != nil {
		t.Errorf("Instance %s is failed to start: %v", TestInstanceId, err)
	}
	t.Logf("Instance %s is running successfully.", TestInstanceId)
	err = client.RebootInstance(TestInstanceId, true)
	if err != nil {
		t.Errorf("Failed to restart instance %s: %v", TestInstanceId, err)
	}
	err = client.WaitForInstance(TestInstanceId, Running, 0)
	if err != nil {
		t.Errorf("Instance %s is failed to restart: %v", TestInstanceId, err)
	}
	t.Logf("Instance %s is running successfully.", TestInstanceId)
}

func TestECSInstanceCreationAndDeletion(t *testing.T) {

	if TestIAmRich == false { // Avoid payment
		return
	}

	client := NewTestClient()
	instance, err := client.DescribeInstanceAttribute(TestInstanceId)
	t.Logf("Instance: %++v  %v", instance, err)

	args := CreateInstanceArgs{
		RegionId:        instance.RegionId,
		ImageId:         instance.ImageId,
		InstanceType:    "ecs.t1.small",
		SecurityGroupId: instance.SecurityGroupIds.SecurityGroupId[0],
	}

	instanceId, err := client.CreateInstance(&args)
	if err != nil {
		t.Errorf("Failed to create instance from Image %s: %v", args.ImageId, err)
	}
	t.Logf("Instance %s is created successfully.", instanceId)

	instance, err = client.DescribeInstanceAttribute(instanceId)
	t.Logf("Instance: %++v  %v", instance, err)

	err = client.WaitForInstance(instanceId, Stopped, 60)

	err = client.StartInstance(instanceId)
	if err != nil {
		t.Errorf("Failed to start instance %s: %v", instanceId, err)
	}
	err = client.WaitForInstance(instanceId, Running, 0)

	err = client.StopInstance(instanceId, true)
	if err != nil {
		t.Errorf("Failed to stop instance %s: %v", instanceId, err)
	}
	err = client.WaitForInstance(instanceId, Stopped, 0)
	if err != nil {
		t.Errorf("Instance %s is failed to stop: %v", instanceId, err)
	}
	t.Logf("Instance %s is stopped successfully.", instanceId)

	err = client.DeleteInstance(instanceId)

	if err != nil {
		t.Errorf("Failed to delete instance %s: %v", instanceId, err)
	}
	t.Logf("Instance %s is deleted successfully.", instanceId)
}

func TestModifyInstanceAttribute(t *testing.T) {
	client := NewTestClient()

	args := ModifyInstanceAttributeArgs{
		InstanceId: TestInstanceId,
		Password:   "Just$test",
	}

	err := client.ModifyInstanceAttribute(&args)
	if err != nil {
		t.Errorf("Failed to modify instance attribute %s: %v", TestInstanceId, err)
	}

	t.Logf("Modify instance attribute successfully")
}

func TestIoOptimized(t *testing.T) {

	type TestStruct struct {
		Str  string
		Flag StringOrBool
	}

	var test TestStruct

	txt := "{\"Str\":\"abc\", \"Flag\": true}"

	err := json.Unmarshal([]byte(txt), &test)
	if err != nil {
		t.Errorf("Failed to Unmarshal IoOptimized: %v", err)
	} else {
		if test.Flag.Value != true {
			t.Errorf("Failed to Unmarshal IoOptimized with expected value: %s", test.Flag)
		}
	}

	txt1 := "{\"Str\":\"abc\", \"Flag\": \"false\"}"

	err = json.Unmarshal([]byte(txt1), &test)
	if err != nil {
		t.Errorf("Failed to Unmarshal IoOptimized: %v", err)
	} else {
		if test.Flag.Value != false {
			t.Errorf("Failed to Unmarshal IoOptimized with expected value: %s", test.Flag)
		}
	}
}

func TestJoinSecurityGroup(t *testing.T) {
	client := NewTestClient()

	err := client.JoinSecurityGroup(TestInstanceId, TestSecurityGroupId)
	if err != nil {
		t.Errorf("Failed to joinSecurityGroup: %v", err)
	}
}

func TestLeaveSecurityGroup(t *testing.T) {
	client := NewTestClient()

	err := client.LeaveSecurityGroup(TestInstanceId, TestSecurityGroupId)
	if err != nil {
		t.Errorf("Failed to LeaveSecurityGroup: %v", err)
	}
}
