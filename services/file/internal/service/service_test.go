package service

import "testing"

func TestCategory(t *testing.T) {
	tests := map[string]string{"image/png": "image", "video/mp4": "video", "audio/ogg": "audio", "application/pdf": "document", "application/zip": "other"}
	for input, want := range tests {
		if got := category(input); got != want {
			t.Errorf("category(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestForbidden(t *testing.T) {
	for _, test := range []struct{ contentType, ext string }{{"application/octet-stream", ".exe"}, {"text/html", ".txt"}, {"image/svg+xml", ".svg"}} {
		if !forbidden(test.contentType, test.ext) {
			t.Errorf("expected %s %s to be forbidden", test.contentType, test.ext)
		}
	}
	if forbidden("application/pdf", ".pdf") {
		t.Fatal("pdf should be allowed")
	}
}

func TestExecutableMagic(t *testing.T) {
	for _, data := range [][]byte{[]byte("MZpayload"), []byte("\x7fELFpayload"), []byte("#!/bin/sh")} {
		if !executableMagic(data) {
			t.Errorf("expected %q to be executable", data)
		}
	}
	if executableMagic([]byte("plain text")) {
		t.Fatal("plain text should be allowed")
	}
}
