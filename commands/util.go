// Copyright 2015-present Oursky Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	skycontainer "github.com/oursky/skycli/container"
	"github.com/spf13/cobra"
)

func checkMinArgCount(cmd *cobra.Command, args []string, min int) {
	if len(args) < min {
		cmd.Usage()
		os.Exit(1)
	}
}

func checkMaxArgCount(cmd *cobra.Command, args []string, max int) {
	if len(args) > max {
		cmd.Usage()
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}

func warn(err error) {
	fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
}

func printValue(value interface{}) {
	switch value.(type) {
	case []interface{}:
		data, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		fmt.Println("%s\n", data)
	case map[string]interface{}:
		data, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\n", data)
	default:
		fmt.Printf("%v\n", value)
	}
}

func usingDatabaseID(c *skycontainer.Container) string {
	if recordUsePrivateDatabase {
		return c.PrivateDatabaseID()
	}
	return c.PublicDatabaseID()
}

func newDatabase() *skycontainer.Database {
	c := newContainer()
	return &skycontainer.Database{
		Container:  c,
		DatabaseID: usingDatabaseID(c),
	}
}
