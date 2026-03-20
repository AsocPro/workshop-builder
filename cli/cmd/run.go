package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/asocpro/workshop-builder/cli/docker"
	"github.com/asocpro/workshop-builder/cli/management"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <image>",
	Short: "Run a workshop locally",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkshop,
}

func init() {
	runCmd.Flags().Bool("no-browser", false, "Do not open the browser automatically")
}

func runWorkshop(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	image := args[0]

	// Create Docker client
	dc, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer dc.Close()

	// Find free ports
	workshopPort, err := freePort()
	if err != nil {
		return fmt.Errorf("finding free port for workshop: %w", err)
	}
	mgmtPort, err := freePort()
	if err != nil {
		return fmt.Errorf("finding free port for management: %w", err)
	}

	// Management URL is consumed by the browser (via the frontend's /api/state),
	// not by the container backend itself, so localhost is correct here.
	mgmtURL := fmt.Sprintf("http://localhost:%d", mgmtPort)
	workshopURL := fmt.Sprintf("http://localhost:%d", workshopPort)

	// Read workshop.json from the image to populate the step list
	fmt.Printf("Reading workshop metadata from %s...\n", image)
	rawSteps, err := dc.ReadWorkshopSteps(ctx, image)
	if err != nil {
		log.Printf("warning: reading workshop.json: %v — step list will be empty", err)
	}
	steps := make([]management.Step, len(rawSteps))
	for i, s := range rawSteps {
		steps[i] = management.Step{ID: s.ID, Title: s.Title}
	}

	// Determine the starting step from the image tag.
	// If the tag matches a known step ID use it; otherwise (e.g. :latest) default
	// to the first step in workshop.json.
	currentStep := ""
	if idx := strings.LastIndex(image, ":"); idx != -1 {
		tag := image[idx+1:]
		for _, s := range steps {
			if s.ID == tag {
				currentStep = tag
				break
			}
		}
	}
	if currentStep == "" && len(steps) > 0 {
		currentStep = steps[0].ID
	}

	// Start management server (host-side, survives container replacements)
	mgmtSrv := management.NewServer(management.Config{
		Port:         mgmtPort,
		WorkshopPort: workshopPort,
		MgmtURL:      mgmtURL,
		WorkshopURL:  workshopURL,
		DC:           dc,
		Image:        image,
		Steps:        steps,
		CurrentStep:  currentStep,
	})
	mgmtSrv.Start()

	// Run the workshop container
	containerID, err := dc.RunContainer(ctx, docker.RunOptions{
		Image:         image,
		Name:          docker.GenerateName("workshop-workspace"),
		WorkshopPort:  workshopPort,
		ManagementURL: mgmtURL,
	})
	if err != nil {
		return fmt.Errorf("starting workshop container: %w", err)
	}
	mgmtSrv.SetCurrentContainer(containerID)
	mgmtSrv.SetCurrentStep(currentStep)

	fmt.Printf("\nWorkshop running at  %s\n", workshopURL)
	fmt.Printf("Management at        http://localhost:%d\n", mgmtPort)
	fmt.Println("Press Ctrl-C to stop.")

	noBrowser, _ := cmd.Flags().GetBool("no-browser")
	if !noBrowser {
		if err := exec.Command("xdg-open", workshopURL).Start(); err != nil {
			log.Printf("could not open browser: %v", err)
		}
	}

	// Wait for Ctrl-C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nStopping…")

	// Stop the container. dc.Close() (deferred above) shuts down the podman
	// socket service, so we must stop the container first while the socket is live.
	currentID := mgmtSrv.CurrentContainer()
	if currentID != "" {
		if err := dc.StopContainer(ctx, currentID); err != nil {
			fmt.Fprintf(os.Stderr, "\nError: could not stop workshop container automatically: %v\n", err)
			fmt.Fprintf(os.Stderr, "You may have a running container that needs manual cleanup:\n")
			fmt.Fprintf(os.Stderr, "  podman stop %s\n", currentID[:12])
			fmt.Fprintf(os.Stderr, "  podman rm   %s\n", currentID[:12])
		}
	}

	return nil
}

// freePort finds an available TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
