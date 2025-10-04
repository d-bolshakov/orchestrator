package cmd

import (
	"log"

	"github.com/d-bolshakov/orchestrator/client"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running task",
	Long: `orchestrator stop command.
	
The stop command stops a running task.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		manager, _ := cmd.Flags().GetString("manager")

		client := client.New(manager, "manager")
		err := client.StopTask(args[0])
		if err != nil {
			log.Fatalf("Error sending request for stopping the task: %v", err)
		}

		log.Printf("Task %v has been stopped.", args[0])
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().StringP("manager", "m", "localhost:5555", "Manager to talk to")
}
