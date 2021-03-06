// Copyright (2012) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.

package main

import (
	"fmt"
	"minicli"
	log "minilog"
	"os"
	"path/filepath"
	"strconv"
)

var (
	// current log level
	logLevel log.Level
	// file that we are currently logging to
	logFile *os.File
)

var logCLIHandlers = []minicli.Handler{
	{ // log level
		HelpShort: "set or print the log level",
		HelpLong: `
Set the log level to one of [debug,info,warn,error,fatal]. Log levels inherit
lower levels, so setting the level to error will also log fatal, and setting
the mode to debug will log everything.`,
		Patterns: []string{
			"log level [debug,info,warn,error,fatal]",
		},
		Call: wrapSimpleCLI(cliLogLevel),
	},
	{ // log stderr
		HelpShort: "enable or disable logging to stderr",
		Patterns: []string{
			"log stderr [true,false]",
		},
		Call: wrapSimpleCLI(cliLogStderr),
	},
	{ // log file
		HelpShort: "enable logging to a file",
		HelpLong: `
Log to a file. To disable file logging, call "clear log file".`,
		Patterns: []string{
			"log file [file]",
		},
		Call: wrapSimpleCLI(cliLogFile),
	},
	{ // log filter
		HelpShort: "filter logging messages",
		HelpLong: `
Control what data gets logged based on matching text. For example, to filter
out all logging messages containing the word "foo":

	log filter foo`,
		Patterns: []string{
			"log filter [filter]",
		},
		Call: wrapSimpleCLI(cliLogFilter),
	},
	{ // log syslog
		HelpShort: "log to syslog",
		HelpLong: `
Log to a syslog daemon on the provided network and address. For example, to log
over UDP to a syslog server foo on port 514:

	log syslog udp foo:514`,
		Patterns: []string{
			"log syslog remote <tcp,udp> <address>",
			"log syslog <local,>",
		},
		Call: wrapSimpleCLI(cliLogSyslog),
	},
	{ // clear log
		HelpShort: "reset state for logging",
		HelpLong: `
Resets state for logging. See "help log ..." for more information.`,
		Patterns: []string{
			"clear log",
			"clear log <file,>",
			"clear log <level,>",
			"clear log <stderr,>",
			"clear log <filter,>",
			"clear log <syslog,>",
		},
		Call: wrapSimpleCLI(cliLogClear),
	},
}

func cliLogLevel(c *minicli.Command, resp *minicli.Response) error {
	if len(c.BoolArgs) == 0 {
		// Print the level
		resp.Response = logLevel.String()
		return nil
	}

	// Bool args should only have a single key that is the log level
	for k := range c.BoolArgs {
		level, _ := log.ParseLevel(k)

		logLevel = level
		log.SetLevelAll(level)
	}

	return nil
}

func cliLogStderr(c *minicli.Command, resp *minicli.Response) error {
	if c.BoolArgs["false"] {
		// Turn off logging to stderr
		log.DelLogger("stderr")
	} else if len(c.BoolArgs) == 0 {
		// Print true or false depending on whether stderr is enabled
		_, err := log.GetLevel("stderr")
		resp.Response = strconv.FormatBool(err == nil)
	} else if c.BoolArgs["true"] {
		// Enable stderr logging if not already enabled
		if _, err := log.GetLevel("stderr"); err != nil {
			log.AddLogger("stderr", os.Stderr, logLevel, true)
		}
	}

	return nil
}

func cliLogFile(c *minicli.Command, resp *minicli.Response) error {
	if len(c.StringArgs) == 0 {
		// Print true or false depending on whether file is enabled
		if logFile != nil {
			resp.Response = logFile.Name()
		}

		return nil
	}

	// Enable logging to file if it's not already enabled
	if logFile != nil {
		if err := stopFileLogger(); err != nil {
			return err
		}
	}

	err := os.MkdirAll(filepath.Dir(c.StringArgs["file"]), 0755)
	if err != nil {
		return err
	}

	flags := os.O_WRONLY | os.O_APPEND | os.O_CREATE
	logFile, err = os.OpenFile(c.StringArgs["file"], flags, 0660)
	if err != nil {
		return err
	}

	log.AddLogger("file", logFile, logLevel, false)
	return nil
}

func cliLogSyslog(c *minicli.Command, resp *minicli.Response) error {
	var network string
	var address string

	if c.BoolArgs["local"] {
		network = "local"
	} else {
		address = c.StringArgs["address"]
		if c.BoolArgs["tcp"] {
			network = "tcp"
		} else {
			network = "udp"
		}
	}

	return log.AddSyslog(network, address, "minimega", logLevel)
}

func cliLogFilter(c *minicli.Command, resp *minicli.Response) error {
	if len(c.StringArgs) == 0 {
		var filters []string
		loggers := log.Loggers()

		for _, l := range loggers {
			filt, _ := log.Filters(l)
			for _, f := range filt {
				var found bool
				for _, v := range filters {
					if v == f {
						found = true
					}
				}
				if !found {
					filters = append(filters, f)
				}
			}
		}

		if len(filters) != 0 {
			resp.Response = fmt.Sprintf("%v", filters)
		}

		return nil
	}

	filter := c.StringArgs["filter"]

	for _, l := range log.Loggers() {
		err := log.AddFilter(l, filter)
		if err != nil {
			return err
		}
	}

	return nil
}

func cliLogClear(c *minicli.Command, resp *minicli.Response) error {
	// Reset file if explicitly cleared or we're clearing everything
	if c.BoolArgs["file"] || len(c.BoolArgs) == 0 {
		if err := stopFileLogger(); err != nil {
			return err
		}
	}

	// Reset syslog if explicitly cleared or we're clearing everything
	if c.BoolArgs["syslog"] || len(c.BoolArgs) == 0 {
		log.DelLogger("syslog")
	}

	// Reset level if explicitly cleared or we're clearing everything
	if c.BoolArgs["level"] || len(c.BoolArgs) == 0 {
		// Reset to level from command line flags
		logLevel = log.LevelFlag

		log.SetLevelAll(logLevel)
	}

	// Reset stderr if explicitly cleared or we're clearing everything
	if c.BoolArgs["stderr"] || len(c.BoolArgs) == 0 {
		// Delete logger to stdout
		log.DelLogger("stderr")
	}

	if c.BoolArgs["filter"] || len(c.BoolArgs) == 0 {
		loggers := log.Loggers()

		for _, l := range loggers {
			filt, _ := log.Filters(l)
			for _, f := range filt {
				log.DelFilter(l, f)
			}
		}
	}

	return nil
}

// stopFileLogger gets rid of the previous file logger
func stopFileLogger() error {
	log.DelLogger("file")

	// no op
	if logFile == nil {
		return nil
	}

	err := logFile.Close()
	if err != nil {
		log.Error("error closing log file: %v", err)
	} else {
		logFile = nil
	}

	return err
}
