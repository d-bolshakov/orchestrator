package cmd

import (
	"log"

	"github.com/d-bolshakov/orchestrator/manager"
	"github.com/spf13/cobra"
)

// managerCmd represents the manager command
var managerCmd = &cobra.Command{
	Use:   "manager",
	Short: "Manager command to operate a manager node",
	Long: `orchestrator manager command
	
The manager controls the orchestration system and is responsible for:
 - Accepting tasks from users
 - Scheduling tasks onto worker nodes
 - Rescheduling tasks in the event of a node failure
 - Periodically polling workers to get task updates`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		workers, _ := cmd.Flags().GetStringSlice("workers")
		schedulerType, _ := cmd.Flags().GetString("scheduler")
		dbType, _ := cmd.Flags().GetString("dbtype")

		log.Println("Starting manager.")
		log.Printf("Workers: %v\n", workers)

		m := manager.New(workers, schedulerType, dbType)
		api := manager.Api{Address: host, Port: port, Manager: m}
		go m.ProcessTasks()
		go m.UpdateTasks()
		go m.DoHeathChecks()
		log.Printf("Starting manager API on http://%s:%d", host, port)
		api.Start()
	},
}

func init() {
	rootCmd.AddCommand(managerCmd)

	managerCmd.Flags().StringP("host", "H", "0.0.0.0", "Hostname or IP address")
	managerCmd.Flags().IntP("port", "p", 5555, "Port on which to listen")
	managerCmd.Flags().StringSliceP("workers", "w", []string{"localhost:5556"}, "List of workers on which the manager will schedule tasks.")
	managerCmd.Flags().StringP("scheduler", "s", "epvm", "Name of scheduler to use.")
	managerCmd.Flags().StringP("dbtype", "d", "inmemory", "Type of datastore to use for events and tasks (\"inmemory\" or \"persistent\")")
}
