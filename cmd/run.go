package cmd

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/d-bolshakov/orchestrator/client"
	"github.com/d-bolshakov/orchestrator/task"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a new task",
	Long: `orchestrator run command.
	
The run command starts a new task.`,
	Run: func(cmd *cobra.Command, args []string) {
		manager, _ := cmd.Flags().GetString("manager")
		filename, _ := cmd.Flags().GetString("filename")

		fullFilePath, err := filepath.Abs(filename)
		if err != nil {
			log.Fatal(err)
		}

		if !fileExists(fullFilePath) {
			log.Fatalf("File %s does not exist.", filename)
		}

		log.Printf("Using manager: %v\n", manager)

		data, err := os.ReadFile(filename)
		if err != nil {
			log.Fatalf("Unable to read file: %v", filename)
		}
		log.Printf("Data: %v\n", string(data))

		var te task.TaskEvent
		err = json.Unmarshal(data, &te)
		if err != nil {
			log.Fatalf("Error unmarshalling task to run: %v", err)
		}

		client := client.New(manager, "manager")
		_, err = client.SendTask(te)
		if err != nil {
			log.Fatalf("Error sending task to the manager: %v\n", err)
		}

		log.Println("Successfully sent task request to manager")
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringP("manager", "m", "localhost:5555", "Manager to talk to")
	runCmd.Flags().StringP("filename", "f", "task.json", "Task specification file")
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)

	return !errors.Is(err, fs.ErrNotExist)
}
