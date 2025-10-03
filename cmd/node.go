package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/d-bolshakov/orchestrator/node"
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Node command to list nodes.",
	Long: `orchestrator node command.
	
The node command allows a user to get the information about the nodes in the cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		manager, _ := cmd.Flags().GetString("manager")

		url := fmt.Sprintf("http://%s/nodes", manager)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("Error connecting to %s: %v", manager, err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			log.Panicf("Error retrieving the node list from %s: %v", manager, err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var nodes []*node.Node
		json.Unmarshal(body, &nodes)

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
