package help

// OpusClip builds the canonical landing screen for the opusclip CLI. version is
// the short version string (e.g. "v1.2.0" or "dev"); pass it from the version
// package so the screen stays in sync with the binary.
func OpusClip(version string) Screen {
	return Screen{
		Version: version,
		Groups: []Group{
			{
				Title: "Commands",
				Commands: []Command{
					{"opusclip auth login", "Authenticate with your OpusClip API key"},
					{"opusclip clip create", "Submit a video URL for clipping"},
					{"opusclip clip watch", "Poll a project through its render stages"},
					{"opusclip clips list", "List a project's generated clips"},
					{"opusclip clips download", "Download the rendered mp4 clips"},
					{"opusclip api", "Make a raw authenticated API request"},
					{"opusclip config", "View and edit configuration"},
					{"opusclip doctor", "Check connectivity and credentials"},
				},
			},
		},
		Footer: "Run `opusclip <command> --help` for details on any command.",
	}
}
