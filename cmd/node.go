package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/d-bolshakov/orchestrator/client"
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Node command to list nodes.",
	Long: `orchestrator node command.
	
The node command allows a user to get the information about the nodes in the cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		manager, _ := cmd.Flags().GetString("manager")

		client := client.NewManagerClient(manager)
		nodes, err := client.GetNodes()
		if err != nil {
			log.Fatalf("Error retrieving the list of nodes from the manager: %v", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 5, ' ', tabwriter.TabIndent)
		fmt.Fprintf(w, "NAME\tMEMORY (MiB)\tDISK (GiB)\tROLE\tTASKS\t\n")
		for _, node := range nodes {
			fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%d\t\n", node.Name, node.Memory/1000, node.Disk/1000/1000/1000, node.Role, node.TaskCount)
		}
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)

	nodeCmd.Flags().StringP("manager", "m", "localhost:5555", "Manager to talk to")
}
