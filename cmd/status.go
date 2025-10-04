package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/d-bolshakov/orchestrator/client"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Status command to list tasks.",
	Long: `orchestrator status command.
	
The status command allows a user to get the status of tasks from the manager.`,
	Run: func(cmd *cobra.Command, args []string) {
		manager, _ := cmd.Flags().GetString("manager")

		client := client.New(manager, "manager")
		tasks, err := client.GetTasks()
		if err != nil {
			log.Fatalf("Error retrieving the task list from the manager: %v", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 5, ' ', tabwriter.TabIndent)
		fmt.Fprintln(w, "ID\tNAME\tCREATED\tSTATE\tCONTAINERNAME\tIMAGE\t")
		for _, task := range tasks {
			var start string
			if task.StartTime.IsZero() {
				start = fmt.Sprintf("%s ago", units.HumanDuration(time.Now().UTC().Sub(time.Now().UTC())))
			} else {
				start = fmt.Sprintf("%s ago", units.HumanDuration(time.Now().UTC().Sub(task.StartTime)))
			}

			state := task.State.String()

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t\n", task.ID, task.Name, start, state, task.Name, task.Image)
		}
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringP("manager", "m", "localhost:5555", "Manager to talk to")
}
