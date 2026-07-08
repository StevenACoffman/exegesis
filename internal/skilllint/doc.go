// Package skilllint is a native Go linter for agent-skill directories. It
// reimplements the checks of the Python skillscheck tool (spec / quality /
// disclosure / agent-compatibility) plus the book2skill Quality Red Lines, so
// exegesis can validate skills without shelling out to an external process (it
// replaced an earlier wrapper around the `uvx skillcheck` binary). The check
// functions here are pure where possible: they take a parsed Skill and return
// diagnostics; filesystem access is confined to a thin shell.
package skilllint
