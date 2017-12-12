package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/alibaba/pouch/ctrd"
	"github.com/alibaba/pouch/pkg/reference"

	"github.com/containerd/containerd/progress"
	"github.com/spf13/cobra"
)

// pullDescription is used to describe pull command in detail and auto generate command doc.
var pullDescription = "Pull an image or a repository from a registry. " +
	"Most of your images will be created on top of a base image from the registry. " +
	"So, you can pull and try prebuilt images contained by registry without needing to define and configure your own."

// PullCommand use to implement 'pull' command, it download image.
type PullCommand struct {
	baseCommand
}

// Init initialize pull command.
func (p *PullCommand) Init(c *Cli) {
	p.cli = c

	p.cmd = &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull an image from registry",
		Long:  pullDescription,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return p.runPull(args)
		},
		Example: pullExample(),
	}
	p.addFlags()
}

// addFlags adds flags for specific command.
func (p *PullCommand) addFlags() {
	// TODO: add flags here
}

// runPull is the entry of pull command.
func (p *PullCommand) runPull(args []string) error {
	ref, err := reference.Parse(args[0])
	if err != nil {
		return fmt.Errorf("failed to pull image: %v", err)
	}

	apiClient := p.cli.Client()
	responseBody, err := apiClient.ImagePull(ref.Name, ref.Tag)
	if err != nil {
		return fmt.Errorf("failed to pull image: %v", err)
	}
	defer responseBody.Close()

	return renderOutput(responseBody)
}

// renderOutput draws the commandline output via api response.
func renderOutput(responseBody io.ReadCloser) error {
	var (
		start = time.Now()
		fw    = progress.NewWriter(os.Stdout)
	)

	dec := json.NewDecoder(responseBody)
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("failed to read the opening token: %v", err)
	}

	for dec.More() {
		var objs []ctrd.ProgressInfo

		tw := tabwriter.NewWriter(fw, 1, 8, 1, ' ', 0)

		if err := dec.Decode(&objs); err != nil {
			return fmt.Errorf("failed to decode: %v", err)
		}

		if err := display(tw, objs, start); err != nil {
			return err
		}

		tw.Flush()
		fw.Flush()
	}

	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("failed to read the closing token: %v", err)
	}
	return nil
}

func display(w io.Writer, statuses []ctrd.ProgressInfo, start time.Time) error {
	var total int64
	for _, status := range statuses {
		if status.ErrorMessage != "" {
			return fmt.Errorf(status.ErrorMessage)
		}
		total += status.Offset
		switch status.Status {
		case "downloading", "uploading":
			var bar progress.Bar
			if status.Total > 0.0 {
				bar = progress.Bar(float64(status.Offset) / float64(status.Total))
			}
			fmt.Fprintf(w, "%s:\t%s\t%40r\t%8.8s/%s\t\n",
				status.Ref,
				status.Status,
				bar,
				progress.Bytes(status.Offset), progress.Bytes(status.Total))

		case "resolving", "waiting":
			bar := progress.Bar(0.0)
			fmt.Fprintf(w, "%s:\t%s\t%40r\t\n",
				status.Ref,
				status.Status,
				bar)

		default:
			bar := progress.Bar(1.0)
			fmt.Fprintf(w, "%s:\t%s\t%40r\t\n",
				status.Ref,
				status.Status,
				bar)
		}
	}

	fmt.Fprintf(w, "elapsed: %-4.1fs\ttotal: %7.6v\t(%v)\t\n",
		time.Since(start).Seconds(),
		progress.Bytes(total),
		progress.NewBytesPerSecond(total, time.Since(start)))
	return nil
}

// pullExample shows examples in pull command, and is used in auto-generated cli docs.
func pullExample() string {
	return `$ pouch images
IMAGE ID            IMAGE NAME                           SIZE
bbc3a0323522        docker.io/library/busybox:latest     2699
$ pouch pull docker.io/library/redis:alpine
$ pouch images
IMAGE ID            IMAGE NAME                           SIZE
bbc3a0323522        docker.io/library/busybox:latest     2699
0153c5db97e5        docker.io/library/redis:alpine       2035`
}
