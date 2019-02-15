package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func runValidateSigstrTest(sigstr string, signalsSupported, isLcow bool,
	expectedSignal int, expectedError bool, t *testing.T) {
	signal, err := validateSigstr(sigstr, signalsSupported, isLcow)
	if expectedError {
		if err == nil {
			t.Fatalf("Expected err: %v, got: nil", expectedError)
		} else if err.Error() != fmt.Sprintf("invalid signal '%s'", sigstr) {
			t.Fatalf("Expected err: %v, got: %v", expectedError, err)
		}
	}
	if signal != expectedSignal {
		t.Fatalf("Test - Signal: %s, Support: %v, LCOW: %v\nExpected signal: %v, got: %v",
			sigstr, signalsSupported, isLcow,
			expectedSignal, signal)
	}
}

func Test_ValidateSigstr_Empty(t *testing.T) {
	runValidateSigstrTest("", false, false, 0, false, t)
	runValidateSigstrTest("", false, true, 0xf, false, t)
	runValidateSigstrTest("", true, false, 0, false, t)
	runValidateSigstrTest("", true, true, 0xf, false, t)
}

func Test_ValidateSigstr_LCOW_NoSignalSupport_Default(t *testing.T) {
	runValidateSigstrTest("15", false, true, 0, false, t)
	runValidateSigstrTest("TERM", false, true, 0, false, t)
	runValidateSigstrTest("SIGTERM", false, true, 0, false, t)
}

func Test_ValidateSigstr_LCOW_NoSignalSupport_Default_Invalid(t *testing.T) {
	runValidateSigstrTest("2", false, true, 0, true, t)
	runValidateSigstrTest("test", false, true, 0, true, t)
}

func Test_ValidateSigstr_WCOW_NoSignalSupport_Default(t *testing.T) {
	runValidateSigstrTest("15", false, false, 0, false, t)
	runValidateSigstrTest("TERM", false, false, 0, false, t)
	runValidateSigstrTest("0", false, false, 0, false, t)
	runValidateSigstrTest("CTRLC", false, false, 0, false, t)
	runValidateSigstrTest("9", false, false, 0, false, t)
	runValidateSigstrTest("KILL", false, false, 0, false, t)
}

func Test_ValidateSigstr_WCOW_NoSignalSupport_Default_Invalid(t *testing.T) {
	runValidateSigstrTest("2", false, false, 0, true, t)
	runValidateSigstrTest("test", false, false, 0, true, t)
}

func Test_ValidateSigstr_LCOW_SignalSupport_SignalNames(t *testing.T) {
	for k, v := range signalMapLcow {
		runValidateSigstrTest(k, true, true, v, false, t)
		// run it again with a case not in the map
		lc := strings.ToLower(k)
		if k == lc {
			t.Fatalf("Expected lower casing - map: %v, got: %v", k, lc)
		}
		runValidateSigstrTest(lc, true, true, v, false, t)
	}
}

func Test_ValidateSigstr_WCOW_SignalSupport_SignalNames(t *testing.T) {
	for k, v := range signalMapWindows {
		runValidateSigstrTest(k, true, false, v, false, t)
		// run it again with a case not in the map
		lc := strings.ToLower(k)
		if k == lc {
			t.Fatalf("Expected lower casing - map: %v, got: %v", k, lc)
		}
		runValidateSigstrTest(lc, true, false, v, false, t)
	}
}

func Test_ValidateSigstr_LCOW_SignalSupport_SignalValues(t *testing.T) {
	for _, v := range signalMapLcow {
		str := strconv.Itoa(v)
		runValidateSigstrTest(str, true, true, v, false, t)
	}
}

func Test_ValidateSigstr_WCOW_SignalSupport_SignalValues(t *testing.T) {
	for _, v := range signalMapWindows {
		str := strconv.Itoa(v)
		runValidateSigstrTest(str, true, false, v, false, t)
	}
}

func Test_ValidateSigstr_WCOW_SignalSupport_Docker_SignalNames(t *testing.T) {
	// Docker KILL -> CTRLSHUTDOWN when signals are supported
	runValidateSigstrTest("KILL", true, false, 0x6, false, t)

	// Docker TERM -> CTRLSHUTDOWN when signals are supported
	runValidateSigstrTest("TERM", true, false, 0x0, false, t)
}

func Test_ValidateSigstr_WCOW_SignalSupport_Docker_SignalValues(t *testing.T) {
	// Docker KILL -> CTRLSHUTDOWN when signals are supported
	runValidateSigstrTest("9", true, false, 0x6, false, t)

	// Docker TERM -> CTRLSHUTDOWN when signals are supported
	runValidateSigstrTest("15", true, false, 0x0, false, t)
}
