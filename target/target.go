package target

type FilesystemTarget struct {
	Name                   string
	Cmd                    string
	Outputs                []string
	Dependencies           []string
	DependencySyncPatterns []string
	TargetDeps             []string
	InputHash              string
	OutputHash             string
	IsPartial              bool // if the job mutates a file that the user might change manually after
}
