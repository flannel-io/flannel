package lcow

//func debugCommand(s string) string {
//	return fmt.Sprintf(`echo -e 'DEBUG COMMAND: %s\\n--------------\\n';%s;echo -e '\\n\\n';`, s, s)
//}

// DebugLCOWGCS extracts logs from the GCS in LCOW. It's a useful hack for debugging,
// but not necessarily optimal, but all that is available to us in RS3.
//func (container *container) DebugLCOWGCS() {
//	if logrus.GetLevel() < logrus.DebugLevel || len(os.Getenv("HCSSHIM_LCOW_DEBUG_ENABLE")) == 0 {
//		return
//	}

//	var out bytes.Buffer
//	cmd := os.Getenv("HCSSHIM_LCOW_DEBUG_COMMAND")
//	if cmd == "" {
//		cmd = `sh -c "`
//		cmd += debugCommand("kill -10 `pidof gcs`") // SIGUSR1 for stackdump
//		cmd += debugCommand("ls -l /tmp")
//		cmd += debugCommand("cat /tmp/gcs.log")
//		cmd += debugCommand("cat /tmp/gcs/gcs-stacks*")
//		cmd += debugCommand("cat /tmp/gcs/paniclog*")
//		cmd += debugCommand("ls -l /tmp/gcs")
//		cmd += debugCommand("ls -l /tmp/gcs/*")
//		cmd += debugCommand("cat /tmp/gcs/*/config.json")
//		cmd += debugCommand("ls -lR /var/run/gcsrunc")
//		cmd += debugCommand("cat /tmp/gcs/global-runc.log")
//		cmd += debugCommand("cat /tmp/gcs/*/runc.log")
//		cmd += debugCommand("ps -ef")
//		cmd += `"`
//	}

//	proc, _, err := container.CreateProcessEx(
//		&CreateProcessEx{
//			OCISpecification: &specs.Spec{
//				Process: &specs.Process{Args: []string{cmd}},
//				Linux:   &specs.Linux{},
//			},
//			CreateInUtilityVm: true,
//			Stdout:            &out,
//		})
//	defer func() {
//		if proc != nil {
//			proc.Kill()
//			proc.Close()
//		}
//	}()
//	if err != nil {
//		logrus.Debugln("benign failure getting gcs logs: ", err)
//	}
//	if proc != nil {
//		proc.WaitTimeout(time.Duration(int(time.Second) * 30))
//	}
//	logrus.Debugf("GCS Debugging:\n%s\n\nEnd GCS Debugging", strings.TrimSpace(out.String()))
//}
