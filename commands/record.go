package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	odcontainer "github.com/oursky/skycli/container"
	odrecord "github.com/oursky/skycli/record"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/twinj/uuid"
)

var handleAsset bool
var assetBaseDirectory string
var promptComplexValue bool
var prettyPrint bool
var recordOutputPath string
var createWhenEdit bool
var recordUsePrivateDatabase bool

func usingDatabaseID(c *odcontainer.Container) string {
	if recordUsePrivateDatabase {
		return c.PrivateDatabaseID()
	}
	return c.PublicDatabaseID()
}

func newDatabase() *odcontainer.Database {
	c := newContainer()
	return &odcontainer.Database{
		Container:  c,
		DatabaseID: usingDatabaseID(c),
	}
}

func formatRecordError(err odcontainer.SkygearError) error {
	var fmtError error
	if err.ID != "" {
		fmtError = fmt.Errorf("Record %s: %s", err.ID, err.Message)
	} else {
		fmtError = errors.New(err.Message)
	}
	return fmtError
}

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Modify records in database",
	Long:  "record is for modifying records in the database, providing Create, Read, Update and Delete functionality.",
}

var recordImportCmd = &cobra.Command{
	Use:   "import [<path> ...]",
	Short: "Import records to database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("not implemented")
	},
}

var recordExportCmd = &cobra.Command{
	Use:   "export <record_id> [<record_id> ...]",
	Short: "Export records from database",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			cmd.Usage()
			os.Exit(1)
		}
		fmt.Println("not implemented")
	},
}

var recordDeleteCmd = &cobra.Command{
	Use:   "delete <record_id> [<record_id> ...]",
	Short: "Delete Records from database",
	Long:  "Each specified record is deleted from the database.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			cmd.Usage()
			os.Exit(1)
		}

		for _, arg := range args {
			if err := odrecord.CheckRecordID(arg); err != nil {
				fatal(err)
			}
		}

		c := newContainer()

		request := odcontainer.GenericRequest{}
		request.Payload = map[string]interface{}{
			"database_id": usingDatabaseID(c),
			"ids":         args,
		}

		response, err := c.MakeRequest("record:delete", &request)
		if err != nil {
			fatal(err)
		}

		if response.IsError() {
			requestError := response.Error()
			fatal(errors.New(requestError.Message))
		}

		resultArray, ok := response.Payload["result"].([]interface{})
		if !ok {
			fatal(fmt.Errorf("Unexpected server data."))
		}

		for i := range resultArray {
			resultData, ok := resultArray[i].(map[string]interface{})
			if !ok {
				warn(fmt.Errorf("Encountered unexpected server data."))
			}

			if odcontainer.IsError(resultData) {
				serverError := odcontainer.MakeError(resultData)
				warn(formatRecordError(serverError))
			}
		}
	},
}

var recordSetCmd = &cobra.Command{
	Use:   "set <record_id> <key=value> [<key=value> ...]",
	Short: "Set attributes on a record",
	Run: func(cmd *cobra.Command, args []string) {
		checkMinArgCount(cmd, args, 2)

		modifyRecord, err := odrecord.MakeEmptyRecord(args[0])
		if err != nil {
			fatal(err)
		}

		for _, arg := range args[1:] {
			err := modifyRecord.Assign(arg)
			if err != nil {
				fatal(err)
			}
		}

		db := newDatabase()
		err = db.SaveRecord(modifyRecord)
		if err != nil {
			fatal(err)
		}
	},
}

var recordGetCmd = &cobra.Command{
	Use:   "get <record_id> <key>",
	Short: "Get value of a record attribute",
	Run: func(cmd *cobra.Command, args []string) {
		checkMinArgCount(cmd, args, 2)
		recordID := args[0]
		desiredKey := args[1]
		err := odrecord.CheckRecordID(recordID)
		if err != nil {
			fatal(err)
		}

		db := newDatabase()
		record, err := db.FetchRecord(recordID)
		if err != nil {
			fatal(err)
		}

		desiredValue, err := record.Get(desiredKey)
		if err != nil {
			fatal(err)
		}

		printValue(desiredValue)
	},
}

func modifyWithEditor(record *odrecord.Record) error {
	recordBytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile("/tmp", "skycli")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(recordBytes)
	if err != nil {
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	editorCmd := exec.Command(editor, f.Name())
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	err = editorCmd.Run()
	if err != nil {
		return err
	}

	f.Seek(0, 0)

	jsonBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonBytes, record)
	if err != nil {
		return err
	}
	return nil
}

var recordEditCmd = &cobra.Command{
	Use:   "edit (<record_type|<record_id>)",
	Short: "Edit a record",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		recordID := args[0]
		if strings.Contains(recordID, "/") {
			err := odrecord.CheckRecordID(recordID)
			if err != nil {
				fatal(err)
			}
		} else {
			recordID = args[0] + "/" + uuid.NewV4().String()
			createWhenEdit = true
		}

		var record *odrecord.Record
		var err error
		db := newDatabase()
		if createWhenEdit {
			record, _ = odrecord.MakeEmptyRecord(recordID)
		} else {
			record, err = db.FetchRecord(recordID)
			if err != nil {
				fatal(err)
			}
		}

		err = modifyWithEditor(record)
		if err != nil {
			fatal(err)
		}

		err = db.SaveRecord(record)
		if err != nil {
			fatal(err)
		}

	},
}
var recordQueryCmd = &cobra.Command{
	Use:   "query <record_type>",
	Short: "Query records from database",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Usage()
			os.Exit(1)
		}

		recordType := args[0]
		if strings.Contains(recordType, "/") {
			fatal(fmt.Errorf("Record type cannot contain '/'."))
		}

		c := newContainer()

		request := odcontainer.GenericRequest{}
		request.Payload = map[string]interface{}{
			"database_id": usingDatabaseID(c),
			"record_type": recordType,
		}

		response, err := c.MakeRequest("record:query", &request)
		if err != nil {
			fatal(err)
		}

		if response.IsError() {
			requestError := response.Error()
			fatal(errors.New(requestError.Message))
		}

		resultArray, ok := response.Payload["result"].([]interface{})
		if !ok {
			fatal(fmt.Errorf("Unexpected server data."))
		}

		for i := range resultArray {
			resultData, ok := resultArray[i].(map[string]interface{})
			if !ok {
				warn(fmt.Errorf("Encountered unexpected server data."))
			}

			if odcontainer.IsError(resultData) {
				serverError := odcontainer.MakeError(resultData)
				warn(formatRecordError(serverError))
				continue
			}

			printValue(resultData)
		}
	},
}

func init() {
	recordCmd.PersistentFlags().BoolVarP(&recordUsePrivateDatabase, "private", "p", false, "Database. Default is Public.")
	viper.BindPFlag("use_private_database", recordCmd.PersistentFlags().Lookup("private"))

	recordImportCmd.Flags().BoolVarP(&handleAsset, "asset", "a", true, "upload assets")
	recordImportCmd.Flags().StringVarP(&assetBaseDirectory, "basedir", "d", "", "base path for locating files to be uploaded")
	recordImportCmd.Flags().BoolVar(&promptComplexValue, "prompt-complex", true, "prompt when complex value is used")

	recordExportCmd.Flags().BoolVarP(&handleAsset, "asset", "a", true, "download assets")
	recordExportCmd.Flags().StringVarP(&assetBaseDirectory, "basedir", "d", "", "base path for locating files to be downloaded")
	recordExportCmd.Flags().BoolVar(&prettyPrint, "pretty-print", false, "print output in a pretty format")
	recordExportCmd.Flags().StringVarP(&recordOutputPath, "output", "o", "", "Path to save the output to. If not specified, output is printed to stdout with newline delimiter.")
	recordGetCmd.Flags().StringVarP(&recordOutputPath, "output", "o", "", "path to save the output to. If not specified, output is printed to stdout.")
	recordGetCmd.Flags().BoolVarP(&handleAsset, "asset", "a", false, "If value to the key is an asset, download the asset and output the content of the asset.")

	recordEditCmd.Flags().BoolVarP(&createWhenEdit, "new", "n", false, "do not fetch record from database before editing")

	recordQueryCmd.Flags().BoolVarP(&handleAsset, "asset", "a", true, "download assets")
	recordQueryCmd.Flags().StringVarP(&assetBaseDirectory, "basedir", "d", "", "base path for locating files to be downloaded")
	recordQueryCmd.Flags().BoolVar(&prettyPrint, "pretty-print", false, "print output in a pretty format")
	recordQueryCmd.Flags().StringVarP(&recordOutputPath, "output", "o", "", "Path to save the output to. If not specified, output is printed to stdout with newline delimiter.")

	recordCmd.AddCommand(recordImportCmd)
	recordCmd.AddCommand(recordExportCmd)
	recordCmd.AddCommand(recordDeleteCmd)
	recordCmd.AddCommand(recordSetCmd)
	recordCmd.AddCommand(recordGetCmd)
	recordCmd.AddCommand(recordEditCmd)
	recordCmd.AddCommand(recordQueryCmd)
}
