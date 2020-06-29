// snippet-sourceauthor:[tokiwong]
// snippet-sourcedescription:[Retrieves cost and usage metrics for your account]
// snippet-keyword:[Amazon Cost Explorer]
// snippet-keyword:[Amazon CE]
// snippet-keyword:[GetCostAndUsage function]
// snippet-keyword:[Go]
// snippet-sourcesyntax:[go]
// snippet-service:[ce]
// snippet-keyword:[Code Sample]
// snippet-sourcetype:[full-example]
// snippet-sourcedate:[2019-07-09]
/*
   Copyright 2010-2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
   This file is licensed under the Apache License, Version 2.0 (the "License").
   You may not use this file except in compliance with the License. A copy of
   the License is located at
    http://aws.amazon.com/apache2.0/
   This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
   CONDITIONS OF ANY KIND, either express or implied. See the License for the
   specific language governing permissions and limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/aws/aws-sdk-go/service/organizations"

	"flag"
	"time"
)

func main() {
	//measure time
	startTime := time.Now()

	//Initialize a session with the osd-staging-1 profile or any user that has access to the desired info
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: "osd-staging-1",
	})
	if err != nil {
		exitErrorf("Unable to generate session,", err)
	}

	// Create Cost Explorer Service Client
	ce := costexplorer.New(sess)
	//Accessing organizations
	org := organizations.New(sess)

	//Get v4 organizational unit
	v4 := organizations.OrganizationalUnit{
		Id:   aws.String("ou-0wd6-aff5ji37"),
		//Id:   aws.String("ou-0wd6-3321fxfw"), //Test small OU
		//Id:   aws.String("ou-0wd6-k7wulboi"), //slightly larger small OU
		//Id:   aws.String("r-0wd6"), //Test root
	}

	//Store cost of OU
	var cost float64 = 0

	//Set flag pointers
	rPtr := flag.Bool("r", false, "recurse")
	recursivePtr := flag.Bool("recursive", false, "recurse")
	timePtr := flag.String("time", "all", "set time")
	//Parse pointers
	flag.Parse()

	//If -r flag is present, do a DFS postorder traversal and get cost of all accounts under OU
	if *rPtr || *recursivePtr {
		DFS(&v4, org, ce, timePtr, &cost)
	} else {	//Else, get cost of only immediate accounts under OU
		getOUCost(&v4, org, ce, timePtr, &cost)
	}

	fmt.Println("Recursive cost of OU:",cost)

	//End time
	endTime := time.Now()
	fmt.Println("Time of program execution:",endTime.Sub(startTime))
}


//Get cost of accounts from current OU and child OUs
func accountCost(accountID *string, ce *costexplorer.CostExplorer, timePtr *string, cost *float64) {
	//Values
	start := strconv.Itoa(time.Now().Year()-1) + time.Now().Format("-01-") + "01"	//Starting from the 1st of the current month last year i.e. if today is 2020-06-29, then start date is 2019-06-01
	end := time.Now().Format("2006-01-02")
	granularity := "MONTHLY"
	metrics := []string{
		"NetUnblendedCost",
	}

	switch *timePtr {
	case "MTD":
		start = time.Now().Format("2006-01") + "-01"
		end = time.Now().Format("2006-01-02")
	case "YTD":
		start = time.Now().Format("2006") + "-01-01"
		end = time.Now().Format("2006-01-02")
	}

	//Get cost information for chosen account
	result, err := ce.GetCostAndUsage(&costexplorer.GetCostAndUsageInput{
		Filter: &costexplorer.Expression{
			Dimensions: &costexplorer.DimensionValues{
				Key: aws.String("LINKED_ACCOUNT"),
				Values: []*string{
					accountID,
				},
			},
		},
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Granularity: aws.String(granularity),
		Metrics: aws.StringSlice(metrics),
	})
	if err != nil {
		exitErrorf("Unable to generate report, %v", err)
	}

	//var totalCost float64 = 0

	//Loop through month-by-month cost to get total cost
	for month := 0; month < len(result.ResultsByTime); month++ {
		currentCost, err := strconv.ParseFloat(*result.ResultsByTime[month].Total["NetUnblendedCost"].Amount, 64)
		if err != nil {
			exitErrorf("Unable to get cost,", err)
		}
		//totalCost += cost
		*cost += currentCost
	}
}

//Get cost of accounts from current OU
func getOUCost(OU *organizations.OrganizationalUnit, org *organizations.Organizations, ce *costexplorer.CostExplorer, timePtr *string, cost *float64) {
	//var cost float64 = 0
	//Get accounts
	accounts, err := org.ListAccountsForParent(&organizations.ListAccountsForParentInput{
		ParentId:   OU.Id,
	})

	//Populate accountSlice with accounts by looping until accounts.NextToken is null
	for {
		if err != nil {	//Look at this for error handling: https://docs.aws.amazon.com/sdk-for-go/api/service/organizations/#example_Organizations_ListOrganizationalUnitsForParent_shared00
			exitErrorf("Unable to retrieve accounts under OU", err)
		}

		////Increment costs of accounts
		for i := 0; i < len(accounts.Accounts); i++ {
			accountCost(accounts.Accounts[i].Id, ce, timePtr, cost)
		}

		if accounts.NextToken == nil {
			break
		}

		//Get accounts
		accounts, err = org.ListAccountsForParent(&organizations.ListAccountsForParentInput{
			ParentId:   OU.Id,
			NextToken: accounts.NextToken,
		})
	}
}

func DFS(OU *organizations.OrganizationalUnit, org *organizations.Organizations, ce *costexplorer.CostExplorer, timePtr *string, cost *float64) {
	//var cost float64 = 0
	var OUSlice []*organizations.OrganizationalUnit

	//Get child OUs under parent OU
	OUs, err := org.ListOrganizationalUnitsForParent(&organizations.ListOrganizationalUnitsForParentInput{
		ParentId: OU.Id,
	})

	//Populate OUSlice with OUs by looping until OUs.NextToken is null
	for {
		if err != nil {
			exitErrorf("Unable to retrieve child OUs under OU", err)
		}

		//Add OUs to slice
		for childOU := 0; childOU < len(OUs.OrganizationalUnits); childOU++ {
			OUSlice = append(OUSlice,OUs.OrganizationalUnits[childOU])
		}

		if OUs.NextToken == nil {
			break
		}

		OUs, err = org.ListOrganizationalUnitsForParent(&organizations.ListOrganizationalUnitsForParentInput{
			ParentId:  OU.Id,
			NextToken: OUs.NextToken,
		})
	}

	//Loop through all child OUs, get their costs, and store it to cost of current OU
	for _,childOU := range OUSlice {
		DFS(childOU, org, ce, timePtr, cost)
	}

	//Return cost of child OUs + cost of immediate accounts under current OU
	getOUCost(OU, org, ce, timePtr, cost)
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

