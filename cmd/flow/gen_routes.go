package main

import (
	"fmt"
	"os"
	"path/filepath"

	rg "github.com/goflow-framework/flow/internal/generator/routegen"
	"github.com/spf13/cobra"
)

var genRoutesCmd = &cobra.Command{
	Use:   "routes [manifest]",
	Short: "Generate typed routes from a manifest",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manifest := "routes.yml"
		if len(args) > 0 {
			manifest = args[0]
		}
		root := generateTarget
		if root == "" {
			var err error
			root, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		outFlag, _ := cmd.Flags().GetString("out")
		if outFlag == "" {
			outFlag = filepath.Join(root, "app", "router", "routes_gen.go")
		}
		if err := rg.GenerateFromFile(manifest, outFlag); err != nil {
			return err
		}
		fmt.Println("created", outFlag)
		return nil
	},
}

func init() {
	generateCmd.AddCommand(genRoutesCmd)
	genRoutesCmd.Flags().String("out", "", "output file path for generated routes (default: target/app/router/routes_gen.go)")
}
