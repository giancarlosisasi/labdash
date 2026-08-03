// Command labdash is the entry point for the dashboard.
//
// Today it carries only the auth and settings commands; the TUI itself lands
// next. Auth comes first because labdash owns its credentials end to end —
// it logs in, stores, and renews them itself rather than borrowing another
// tool's. See research/09-auth-strategy.md.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/giancarlosisasi/labdash/internal/gitlabauth"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		// cobra has already printed the error.
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "labdash",
		Short:         "A terminal dashboard for GitLab merge requests and pipelines",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.AddCommand(newAuthCmd(), newSettingsCmd())

	return root
}

// ---------------------------------------------------------------------------
// auth
// ---------------------------------------------------------------------------

func newAuthCmd() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Manage GitLab credentials",
		Long: "Manage GitLab credentials.\n\n" +
			"labdash owns its credentials: it logs in, stores them in your OS keyring,\n" +
			"and renews them itself. It does not read glab's configuration or keyring —\n" +
			"borrowing another tool's token meant inheriting an expiry we could not fix.",
	}

	auth.AddCommand(newAuthLoginCmd(), newAuthStatusCmd(), newAuthLogoutCmd())

	return auth
}

func newAuthLoginCmd() *cobra.Command {
	var (
		hostname  string
		clientID  string
		readOnly  bool
		useWeb    bool
		noBrowser bool
		withToken bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a GitLab instance",
		Long: "Authenticate with a GitLab instance and store the credential where only\n" +
			"labdash uses it.\n\n" +
			"The default flow is the OAuth device grant: you get a short code, a browser\n" +
			"opens, and you approve. It needs no callback listener, so it works over SSH,\n" +
			"inside WSL, and in containers. Instances older than GitLab 17.9 fall back to\n" +
			"a browser flow automatically. The token is then refreshed automatically.\n\n" +
			"Self-managed instances need their own OAuth application. If registering one\n" +
			"is not practical, use --with-token and a personal access token instead.\n\n" +
			"A personal access token is a different thing from the OAuth token this\n" +
			"command normally mints: you create it by hand in GitLab's UI, it lasts up to\n" +
			"a year, and nothing renews it. An OAuth token lasts two hours and renews\n" +
			"itself. Prefer OAuth wherever an application exists.\n\n" +
			"  labdash auth login --with-token\n" +
			"      prompts for the token; nothing is echoed as you paste\n\n" +
			"  echo $TOKEN | labdash auth login --with-token\n" +
			"      reads it from a pipe, for scripts and CI\n\n" +
			"The token is never read from a flag, because a flag value lands in shell\n" +
			"history and in the process list.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			cfg, err := gitlabauth.LoadConfig()
			if err != nil {
				return err
			}
			host := cfg.ResolveHost(hostname)
			inst, _ := cfg.Instance(host)

			opts := gitlabauth.LoginOptions{
				Host:      host,
				ClientID:  clientID,
				ReadOnly:  readOnly,
				NoBrowser: noBrowser,
				Instance:  &inst,
				Out:       cmd.OutOrStdout(),
			}
			if useWeb {
				opts.Method = gitlabauth.MethodBrowser
			}

			var creds gitlabauth.Credentials
			if withToken {
				token, err := readToken(cmd.InOrStdin(), cmd.OutOrStdout(), host)
				if err != nil {
					return err
				}
				creds, err = gitlabauth.LoginWithToken(ctx, token, opts)
				if err != nil {
					return err
				}
			} else {
				creds, err = gitlabauth.Login(ctx, opts)
				if err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()

			who := "you"
			if id, err := gitlabauth.WhoAmI(ctx, creds, nil); err == nil {
				if g := id.Greeting(); g != "" {
					who = g
				}
			}

			fmt.Fprintf(out, "\n  Welcome, %s.\n", who)
			fmt.Fprintf(out, "  Logged in to %s.\n", creds.Host)
			fmt.Fprintf(out, "  Stored in %s\n", creds.Source)
			if creds.Kind == gitlabauth.KindOAuth {
				fmt.Fprintf(out, "  It renews itself, so you will not be asked again.\n")
				if !withToken {
					// GitLab's device page re-renders its empty "Device code"
					// form after authorizing, which reads as a second prompt.
					// Say plainly that the browser is finished with.
					fmt.Fprintf(out, "\n  You can close the browser tab. GitLab redisplays its device\n")
					fmt.Fprintf(out, "  form after authorizing, but nothing more is needed there.\n")
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&hostname, "hostname", "",
		"GitLab host (default: settings defaultHost, then $GITLAB_HOST, then gitlab.com)")
	cmd.Flags().StringVar(&clientID, "client-id", "",
		"OAuth application ID; required for self-managed instances")
	cmd.Flags().BoolVar(&readOnly, "read-only", false,
		"request read_api instead of api, so the dashboard cannot merge, retry, or cancel")
	cmd.Flags().BoolVar(&useWeb, "web", false,
		"force the browser flow instead of the device flow")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false,
		"print the code and URL without opening a browser, for SSH and headless machines")
	cmd.Flags().BoolVar(&withToken, "with-token", false,
		"read a personal access token from stdin instead of running an OAuth flow")

	return cmd
}

// readToken takes a personal access token from standard input.
//
// Two shapes, both supported:
//
//	labdash auth login --with-token          # prompts; input is not echoed
//	echo $TOKEN | labdash auth login --with-token   # piped, for scripts
//
// Standard input rather than a --token flag, because a flag value lands in
// shell history and is visible in the process list to every user on the
// machine. When we are on a terminal the input is read with echo off, so the
// token does not stay in the scrollback either.
func readToken(in io.Reader, out io.Writer, host string) (string, error) {
	f, isFile := in.(*os.File)
	interactive := false
	if isFile {
		if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			interactive = true
		}
	}

	if !interactive {
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("reading the token from standard input: %w", err)
		}
		return strings.TrimSpace(line), nil
	}

	fmt.Fprintf(out, "Paste a personal access token for %s, then press Enter.\n", host)
	fmt.Fprintf(out, "Create one at https://%s/-/user_settings/personal_access_tokens"+
		"?scopes=api\n", host)
	fmt.Fprintf(out, "Nothing will appear as you type or paste.\n\n")
	fmt.Fprintf(out, "Token: ")

	raw, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", fmt.Errorf("reading the token: %w", err)
	}

	return strings.TrimSpace(string(raw)), nil
}

func newAuthStatusCmd() *cobra.Command {
	var (
		hostname string
		offline  bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show which credential will be used, and where it came from",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			creds, err := gitlabauth.Resolve(gitlabauth.Options{Host: hostname})
			if err != nil {
				// cobra prints the error itself; printing it here too would
				// show the whole "not logged in" block twice.
				return err
			}

			if !offline {
				ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
				defer cancel()

				if id, err := gitlabauth.WhoAmI(ctx, creds, nil); err == nil {
					fmt.Fprintf(out, "%s\n", id.Greeting())
					if id.Email != "" {
						fmt.Fprintf(out, "  email   : %s\n", id.Email)
					}
				} else {
					fmt.Fprintf(out, "  could not identify the user: %v\n", err)
				}
			}

			fmt.Fprintf(out, "%s\n", creds.Describe())
			fmt.Fprintf(out, "  graphql : %s\n", creds.GraphQLEndpoint())
			fmt.Fprintf(out, "  rest    : %s\n", creds.RESTEndpoint())

			switch {
			case creds.Managed && creds.Kind == gitlabauth.KindOAuth:
				fmt.Fprintf(out, "  expires : %s (renews automatically)\n",
					creds.OAuth2Expiry.Format(time.RFC3339))
			case creds.Managed:
				fmt.Fprintf(out, "  expires : never — personal access tokens do not auto-renew\n")
			default:
				fmt.Fprintf(out, "  source  : an environment variable, not our store\n")
				fmt.Fprintf(out, "            `auth logout` will not remove it; unset the variable\n")
			}

			if advice := creds.Advisory(); advice != "" {
				fmt.Fprintf(out, "\n  note: %s\n", advice)
			}

			fmt.Fprintf(out, "\n  settings: %s\n", gitlabauth.SettingsPath())

			return nil
		},
	}

	cmd.Flags().StringVar(&hostname, "hostname", "",
		"GitLab host (default: settings defaultHost, then $GITLAB_HOST, then gitlab.com)")
	cmd.Flags().BoolVar(&offline, "offline", false,
		"skip the identity lookup and make no network calls")

	return cmd
}

func newAuthLogoutCmd() *cobra.Command {
	var hostname string

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Forget our stored credential for a host",
		Long: "Forget the credential labdash stored for a host.\n\n" +
			"This only forgets the token locally. The token stays valid at GitLab until\n" +
			"it expires; to invalidate it immediately, revoke the application at\n" +
			"https://<host>/-/user_settings/applications.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			cfg, err := gitlabauth.LoadConfig()
			if err != nil {
				return err
			}
			host := cfg.ResolveHost(hostname)

			removed, err := gitlabauth.Logout(host, nil)
			if err != nil {
				return err
			}

			if !removed {
				fmt.Fprintf(out, "No labdash credential was stored for %s — nothing to remove.\n", host)
				fmt.Fprintf(out, "If `auth status` still shows a token, it came from an environment\n")
				fmt.Fprintf(out, "variable; unset that instead.\n")
				return nil
			}

			fmt.Fprintf(out, "Removed the labdash credential for %s.\n\n", host)
			fmt.Fprintf(out, "This only forgot the token locally. It stays valid at GitLab until it\n")
			fmt.Fprintf(out, "expires. To invalidate it now, revoke the application at:\n  %s\n",
				gitlabauth.RevokeURL(host))

			return nil
		},
	}

	cmd.Flags().StringVar(&hostname, "hostname", "",
		"GitLab host (default: settings defaultHost, then $GITLAB_HOST, then gitlab.com)")

	return cmd
}

// ---------------------------------------------------------------------------
// settings
// ---------------------------------------------------------------------------

func newSettingsCmd() *cobra.Command {
	cfg := &cobra.Command{
		Use:   "settings",
		Short: "Inspect labdash's settings",
	}

	cfg.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the path of the settings file",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), gitlabauth.SettingsPath())
		},
	})

	cfg.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print the configured instances (never any credential)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			loaded, err := gitlabauth.LoadConfig()
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "settings : %s\n", gitlabauth.SettingsPath())
			if loaded.DefaultHost != "" {
				fmt.Fprintf(out, "default: %s\n", loaded.DefaultHost)
			}

			if len(loaded.Instances) == 0 {
				fmt.Fprintf(out, "\nNo instances configured. gitlab.com works without any settings;\n")
				fmt.Fprintf(out, "self-managed instances need an entry. Example:\n\n")
				fmt.Fprintf(out, "%s\n", exampleSettings)
				return nil
			}

			for host, inst := range loaded.Instances {
				fmt.Fprintf(out, "\n%s\n", host)
				printIfSet(out, "apiHost", inst.APIHost)
				printIfSet(out, "apiProtocol", inst.APIProtocol)
				printIfSet(out, "subfolder", inst.Subfolder)
				printIfSet(out, "clientId", inst.ClientID)
				printIfSet(out, "tokenEnv", inst.TokenEnv)
				printIfSet(out, "caCert", inst.CACert)
				printIfSet(out, "clientCert", inst.ClientCert)
				if inst.InsecureSkipVerify {
					fmt.Fprintf(out, "  %-12s: true  ← TLS verification is OFF for this host\n",
						"insecureSkipVerify")
				}
				printIfSet(out, "proxy", inst.Proxy)
				if n := len(inst.CustomHeaders); n > 0 {
					fmt.Fprintf(out, "  %-12s: %d\n", "customHeaders", n)
				}
			}

			return nil
		},
	})

	return cfg
}

func printIfSet(out io.Writer, name, value string) {
	if value != "" {
		fmt.Fprintf(out, "  %-12s: %s\n", name, value)
	}
}

const exampleSettings = `defaultHost: gitlab.example.com

instances:
  gitlab.example.com:
    # OAuth application registered on that instance, if you have one.
    clientId: <application id>
    # Or skip OAuth: name an environment variable holding a token.
    tokenEnv: GITLAB_WORK_TOKEN
    # Only if the instance is not at the domain root.
    subfolder: gitlab
    # Only if the API is reached at a different hostname.
    apiHost: api.example.com
    caCert: /etc/ssl/corp-root.pem
    customHeaders:
      - name: Cf-Access-Client-Secret
        valueFromEnv: CF_ACCESS_SECRET`
