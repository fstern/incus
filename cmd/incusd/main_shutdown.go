package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	incus "github.com/lxc/incus/v6/client"
)

type cmdShutdown struct {
	global *cmdGlobal

	flagForce   bool
	flagTimeout int
}

func (c *cmdShutdown) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "shutdown"
	cmd.Short = "Tell the daemon to shutdown all instances and exit"
	cmd.Long = `Description:
  Tell the daemon to shutdown all instances and exit

  This will tell the daemon to start a clean shutdown of all instances,
  followed by having itself shutdown and exit.

  This can take quite a while as instances can take a long time to
  shutdown, especially if a non-standard timeout was configured for them.
`
	cmd.RunE = c.run
	cmd.Flags().IntVarP(&c.flagTimeout, "timeout", "t", 0, "Number of seconds to wait before giving up"+"``")
	cmd.Flags().BoolVarP(&c.flagForce, "force", "f", false, "Force shutdown instead of waiting for running operations to finish"+"``")
	cmd.Hidden = true

	return cmd
}

func (c *cmdShutdown) run(_ *cobra.Command, _ []string) error {
	connArgs := &incus.ConnectionArgs{
		SkipGetServer: true,
	}

	d, err := incus.ConnectIncusUnix("", connArgs)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("force", strconv.FormatBool(c.flagForce))

	chResult := make(chan error, 1)
	go func() {
		defer close(chResult)

		httpClient, err := d.GetHTTPClient()
		if err != nil {
			chResult <- err
			return
		}

		// Request shutdown, this shouldn't return until daemon has stopped so use a large request timeout.
		httpTransport := httpClient.Transport.(*http.Transport)
		httpTransport.ResponseHeaderTimeout = 3600 * time.Second

		_, _, err = d.RawQuery("PUT", fmt.Sprintf("/internal/shutdown?%s", v.Encode()), nil, "")
		if err != nil {
			chResult <- err
			return
		}
	}()

	if c.flagTimeout > 0 {
		select {
		case err = <-chResult:
			return err
		case <-time.After(time.Second * time.Duration(c.flagTimeout)):
			return fmt.Errorf("Daemon still running after %ds timeout", c.flagTimeout)
		}
	}

	return <-chResult
}
