// Offline builder only: no SSH or deployment commands.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const prefix = "generated/de/vw/mib/appui/controlcenter/internal/view/"

func sha(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func readZip(path string) (map[string][]byte, []zip.FileHeader, error) {
	r, e := zip.OpenReader(path)
	if e != nil {
		return nil, nil, e
	}
	defer r.Close()
	data := map[string][]byte{}
	headers := []zip.FileHeader{}
	for _, f := range r.File {
		if _, ok := data[f.Name]; ok {
			return nil, nil, errors.New("duplicate ZIP entry")
		}
		s, e := f.Open()
		if e != nil {
			return nil, nil, e
		}
		b, e := io.ReadAll(s)
		s.Close()
		if e != nil {
			return nil, nil, e
		}
		data[f.Name] = b
		headers = append(headers, f.FileHeader)
	}
	return data, headers, nil
}
func cmd(jdk, name string, args ...string) error {
	c := exec.Command(filepath.Join(jdk, "bin", name), args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
func build() error {
	jdk := flag.String("jdk", os.Getenv("JAVA_HOME"), "JDK17 home")
	src := flag.String("sources", ".", "source directory")
	view := flag.String("view-jar", "", "original viewhandler")
	main := flag.String("hmi-jar", "", "original HMI")
	out := flag.String("output", "", "new output directory")
	flag.Parse()
	if *jdk == "" || *out == "" {
		return errors.New("jdk and output required")
	}
	if _, e := os.Lstat(*out); !os.IsNotExist(e) {
		return errors.New("output already exists")
	}
	vb, e := os.ReadFile(*view)
	if e != nil {
		return e
	}
	mb, e := os.ReadFile(*main)
	if e != nil {
		return e
	}
	if sha(vb) != "9bd5764fabe2cf79e5193f2dd39c38954f6170a8f016c229c133e2b8165a1ce1" || sha(mb) != "c27e37f93b9877c023487af8566e5cb33b9475835fcee6fe1b68f6c8affc7e17" {
		return errors.New("unknown firmware")
	}
	original, headers, e := readZip(*view)
	if e != nil {
		return e
	}
	pinBytes, e := os.ReadFile(filepath.Join(*src, "target-pins.json"))
	if e != nil {
		return e
	}
	pins := map[string]string{}
	if e = json.Unmarshal(pinBytes, &pins); e != nil {
		return e
	}
	if len(pins) != 8 {
		return errors.New("target count differs")
	}
	tmp, e := os.MkdirTemp("", "daynight-build-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(tmp)
	classes := filepath.Join(tmp, "classes")
	os.Mkdir(classes, 0700)
	if e = cmd(*jdk, "javac", "--release", "8", "-cp", *main, "-d", classes, filepath.Join(*src, "java", filepath.FromSlash(prefix+"DayNightSelection.java"))); e != nil {
		return e
	}
	exports := []string{"--add-exports", "java.base/jdk.internal.org.objectweb.asm=ALL-UNNAMED", "--add-exports", "java.base/jdk.internal.org.objectweb.asm.tree=ALL-UNNAMED"}
	if e = cmd(*jdk, "javac", append(append([]string{}, exports...), "-d", classes, filepath.Join(*src, "java", "PatchViews.java"))...); e != nil {
		return e
	}
	changed := map[string][]byte{}
	names := []string{}
	for n := range pins {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		b := append([]byte(nil), original[n]...)
		if sha(b) != pins[n] {
			return errors.New("unrecognized view " + n)
		}
		offset := -1
		if strings.HasSuffix(n, "/Sc__5.class") {
			offset = 7169
		}
		if strings.HasSuffix(n, "/Scm__9.class") {
			offset = 7212
		}
		if offset >= 0 {
			if b[offset] != 3 {
				return errors.New("unexpected visibility opcode")
			}
			b[offset] = 4
		}
		input, output := filepath.Join(tmp, "input.class"), filepath.Join(tmp, "output.class")
		if e = os.WriteFile(input, b, 0600); e != nil {
			return e
		}
		args := append(append([]string{}, exports...), "-cp", classes, "PatchViews", input, output)
		if e = cmd(*jdk, "java", args...); e != nil {
			return e
		}
		changed[n], e = os.ReadFile(output)
		if e != nil {
			return e
		}
	}
	geometries := map[string][][3]int{"Sc__0": {{19610, 280, 400}, {19809, 280, 400}, {20037, 280, 400}}, "Scm__0": {{20820, 280, 400}, {21019, 280, 400}, {21247, 237, 357}}}
	hashes := map[string]string{"Sc__0": "39abf3980eeb4737fbad649ee92787f6b796fc377c6185d38f1d4ef722aac9a3", "Scm__0": "08aca22763be53c8dcff6e942b272142b7cad53b7f25587906979d6f719ba30c"}
	for name, items := range geometries {
		n := prefix + name + ".class"
		b := append([]byte(nil), original[n]...)
		if sha(b) != hashes[name] {
			return errors.New("unknown geometry")
		}
		for _, v := range items {
			at := v[0]
			if b[at] != 0x11 || int(binary.BigEndian.Uint16(b[at+1:])) != v[1] || b[at+3] != 0xb6 {
				return errors.New("geometry opcode mismatch")
			}
			binary.BigEndian.PutUint16(b[at+1:], uint16(v[2]))
		}
		changed[n] = b
	}
	helpers := map[string][]byte{}
	files, _ := filepath.Glob(filepath.Join(classes, filepath.FromSlash(prefix+"DayNightSelection*.class")))
	if len(files) != 4 {
		return errors.New("unexpected helper classes: expected adapter and three API callbacks")
	}
	allowedHelpers := map[string]bool{"DayNightSelection.class": true, "DayNightSelection$1.class": true, "DayNightSelection$1$1.class": true, "DayNightSelection$1$1$1.class": true}
	for _, p := range files {
		if !allowedHelpers[filepath.Base(p)] {
			return errors.New("unrecognized helper entry")
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		helpers[prefix+filepath.Base(p)] = b
	}
	var buffer bytes.Buffer
	w := zip.NewWriter(&buffer)
	for _, h := range headers {
		s, e := w.CreateHeader(&h)
		if e != nil {
			return e
		}
		b, ok := changed[h.Name]
		if !ok {
			b = original[h.Name]
		}
		if _, e = s.Write(b); e != nil {
			return e
		}
	}
	names = nil
	for n := range helpers {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		h := zip.FileHeader{Name: n, Method: zip.Deflate}
		h.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		h.SetMode(0644)
		s, e := w.CreateHeader(&h)
		if e != nil {
			return e
		}
		if _, e = s.Write(helpers[n]); e != nil {
			return e
		}
	}
	if e = w.Close(); e != nil {
		return e
	}
	candidate := filepath.Join(tmp, "candidate.jar")
	os.WriteFile(candidate, buffer.Bytes(), 0600)
	verified, _, e := readZip(candidate)
	if e != nil {
		return e
	}
	if len(verified) != len(original)+len(helpers) {
		return errors.New("entry set changed")
	}
	for n, b := range original {
		if c, ok := changed[n]; ok {
			b = c
		}
		if !bytes.Equal(verified[n], b) {
			return errors.New("scope verification failed: " + n)
		}
	}
	for n, b := range helpers {
		if !bytes.Equal(verified[n], b) {
			return errors.New("helper verification failed")
		}
	}
	if e = os.Mkdir(*out, 0700); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(*out, "viewhandler.candidate.jar"), buffer.Bytes(), 0600); e != nil {
		return e
	}
	report := map[string]any{"vehicle_installed": false, "ui_validated": false, "candidate_sha256": sha(buffer.Bytes()), "changed_classes": len(changed), "added_helper_classes": len(helpers), "runtime": "asynchronous Java menu adapter; Go offline builder only"}
	b, _ := json.MarshalIndent(report, "", "  ")
	if e = os.WriteFile(filepath.Join(*out, "build.json"), b, 0600); e != nil {
		return e
	}
	fmt.Println(string(b))
	return nil
}
func main() {
	if e := build(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
