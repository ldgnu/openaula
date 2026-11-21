// Command openaula configures the AULA/Epomaker F108 Pro keyboard.
//
// With no arguments it opens the interactive TUI. Every action opens the
// device, sends the command and exits: there is no resident process.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ldgnu/openaula/internal/api"
	"github.com/ldgnu/openaula/internal/aula"
	"github.com/ldgnu/openaula/internal/device"
	"github.com/ldgnu/openaula/internal/state"
	"github.com/ldgnu/openaula/internal/tui"
	"github.com/ldgnu/openaula/internal/wal"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	store, err := state.NewStore()
	if err != nil {
		return err
	}
	svc := device.New()

	if len(args) == 0 {
		return tui.Run(svc, store)
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "tui":
		return tui.Run(svc, store)
	case "serve":
		return cmdServe(svc, store, rest)
	case "status":
		return cmdStatus(svc, store, rest)
	case "light":
		return cmdLight(svc, store, rest)
	case "off":
		return cmdLight(svc, store, []string{"--off"})
	case "preset":
		return cmdPreset(svc, store, rest)
	case "os":
		return cmdOS(svc, store, rest)
	case "battery":
		return cmdBattery()
	case "wal":
		return cmdWal(svc, store, rest)
	case "perkey":
		return cmdPerKey(svc, store, rest)
	case "remap":
		return cmdRemap(svc, store, rest)
	case "clock":
		fmt.Println("Sincronizando reloj…")
		if err := svc.SyncClock(); err != nil {
			return err
		}
		fmt.Println("OK")
		return nil
	case "lcd":
		return cmdLCD(svc, rest)
	case "keys":
		return cmdKeys()
	case "modes":
		return cmdModes()
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("comando desconocido %q", cmd)
	}
}

func usage() {
	fmt.Print(`openaula — configuración del AULA F108 Pro

uso:
  openaula                      abre la TUI
  openaula serve [--addr ...]   API HTTP/JSON local (opcional)
  openaula status [--json]      estado del dispositivo
  openaula light [flags]        luz de fondo
  openaula off                  apaga la luz
  openaula preset <id>          aplica un preset de remapeo
  openaula battery              batería (solo dongle 2.4 GHz)
  openaula wal [--watch]        usa el color de pywal
  openaula os [id]              configura modificadores para Windows/Linux, macOS, iOS o Android
  openaula perkey ...           color por tecla
  openaula remap ...            remapeos
  openaula clock                sincroniza el reloj del LCD
  openaula lcd color r g b      pinta el LCD
  openaula lcd image foto.gif   sube un GIF al LCD
  openaula keys                 lista nombres de teclas
  openaula modes                lista modos de luz

La lista de presets: openaula preset --list
`)
}

func cmdStatus(svc *device.Service, store *state.Store, args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "salida JSON")
	_ = fs.Parse(args)

	st, err := svc.Status(store)
	if err != nil {
		return err
	}
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(st)
	}
	fmt.Printf("conexión:     %s\n", st.Device.Transport)
	fmt.Printf("vid:pid:      %04x:%04x\n", st.Device.VID, st.Device.PID)
	fmt.Printf("producto:     %s\n", st.Device.Product)
	fmt.Printf("configurable: %v\n", st.Writable)
	if st.Device.FirmwareVersion() != "" {
		fmt.Printf("firmware:     %s (bcdDevice 0x%04x)\n", st.Device.FirmwareVersion(), st.Device.BCDDevice)
	}
	if st.HasBattery {
		fmt.Printf("batería:      %d%%\n", st.Battery)
	}
	if st.Writable {
		fmt.Printf("modo de luz:  %d\n", st.Settings.Lighting.Mode)
		fmt.Printf("per-key:      %d teclas\n", len(st.Settings.PerKey.Colors))
		fmt.Printf("remaps:       %d normal · %d FN\n", len(st.Settings.Remap.Normal), len(st.Settings.Remap.FN))
		fmt.Printf("sistema:      %s\n", st.Settings.OS)
	}
	return nil
}

func cmdLight(svc *device.Service, store *state.Store, args []string) error {
	fs := flag.NewFlagSet("light", flag.ExitOnError)
	mode := fs.Int("mode", -1, "modo 0-19")
	color := fs.String("color", "", "r,g,b")
	brightness := fs.Int("brightness", -1, "brillo 0-5")
	speed := fs.Int("speed", -1, "velocidad 0-5")
	direction := fs.Int("direction", -1, "dirección 0-1")
	colorful := fs.Bool("colorful", false, "arcoíris")
	off := fs.Bool("off", false, "apagar")
	_ = fs.Parse(args)

	st, err := store.Load()
	if err != nil {
		return err
	}
	l := st.Lighting
	if *off {
		l = state.Lighting{Mode: 0}
	}
	if *mode >= 0 {
		l.Mode = *mode
	}
	if *color != "" {
		var r, g, b uint8
		if _, err := fmt.Sscanf(*color, "%d,%d,%d", &r, &g, &b); err != nil {
			return fmt.Errorf("color inválido %q (usá r,g,b)", *color)
		}
		l.R, l.G, l.B = r, g, b
	}
	if *brightness >= 0 {
		l.Brightness = uint8(*brightness)
	}
	if *speed >= 0 {
		l.Speed = uint8(*speed)
	}
	if *direction >= 0 {
		l.Direction = uint8(*direction)
	}
	if *colorful {
		l.Colorful = true
	}
	if err := svc.ApplyLighting(l); err != nil {
		return err
	}
	st.Lighting = l
	if err := store.Save(st); err != nil {
		return err
	}
	fmt.Println("OK")
	return nil
}

func cmdPreset(svc *device.Service, store *state.Store, args []string) error {
	fs := flag.NewFlagSet("preset", flag.ExitOnError)
	list := fs.Bool("list", false, "lista presets")
	_ = fs.Parse(args)
	if *list || fs.NArg() == 0 {
		for _, p := range device.Presets() {
			fmt.Printf("  %-10s %s\n", p.ID, p.Name)
			fmt.Printf("             %s\n", p.Description)
		}
		return nil
	}
	id := fs.Arg(0)
	st, err := store.Load()
	if err != nil {
		return err
	}
	keys, err := svc.ApplyPreset(id, true)
	if err != nil {
		return err
	}
	for k, v := range keys {
		st.Remap.Normal[k] = v
	}
	if err := store.Save(st); err != nil {
		return err
	}
	fmt.Printf("Preset %s aplicado (%d remapeos)\n", id, len(keys))
	return nil
}

func cmdWal(svc *device.Service, store *state.Store, args []string) error {
	fs := flag.NewFlagSet("wal", flag.ExitOnError)
	index := fs.Int("index", 4, "color de pywal a usar (0-15)")
	brightness := fs.Int("brightness", 5, "brillo 0-5")
	mode := fs.Int("mode", 1, "modo de luz (0-19)")
	watch := fs.Bool("watch", false, "seguir los cambios de pywal")
	_ = fs.Parse(args)

	apply := func() error {
		pal, err := wal.Load()
		if err != nil {
			return err
		}
		c := pal.Accent(*index)
		st, err := store.Load()
		if err != nil {
			return err
		}
		l := st.Lighting
		l.Mode = *mode
		l.R, l.G, l.B = c[0], c[1], c[2]
		l.Brightness = uint8(*brightness)
		if err := svc.ApplyLighting(l); err != nil {
			return err
		}
		st.Lighting = l
		if err := store.Save(st); err != nil {
			return err
		}
		fmt.Printf("pywal color%d -> #%02x%02x%02x\n", *index, c[0], c[1], c[2])
		return nil
	}

	if err := apply(); err != nil {
		return err
	}
	if !*watch {
		return nil
	}

	dir, err := wal.Dir()
	if err != nil {
		return err
	}
	file := dir + "/colors.json"
	last := fileModTime(file)
	for {
		time.Sleep(2 * time.Second)
		cur := fileModTime(file)
		if cur != last {
			last = cur
			if err := apply(); err != nil {
				fmt.Fprintln(os.Stderr, "wal:", err)
			}
		}
	}
}

func fileModTime(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

func cmdBattery() error {
	d, err := aula.OpenDongle()
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Probe(); err != nil {
		return err
	}
	pct, err := d.Battery()
	if err != nil {
		return err
	}
	fmt.Printf("%d%%\n", pct)
	return nil
}

func cmdOS(svc *device.Service, store *state.Store, args []string) error {
	st, err := store.Load()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		for _, p := range device.OSProfiles() {
			mark := " "
			if p.ID == st.OS {
				mark = "*"
			}
			fmt.Printf(" %s %-10s %s (%s)\n", mark, p.ID, p.Name, p.FNCombo)
		}
		return nil
	}
	keys, err := svc.ApplyOSProfile(args[0], st.Remap.Normal)
	if err != nil {
		return err
	}
	st.Remap.Normal = keys
	st.OS = args[0]
	if err := store.Save(st); err != nil {
		return err
	}
	fmt.Printf("Perfil %s aplicado\n", args[0])
	return nil
}

func cmdPerKey(svc *device.Service, store *state.Store, args []string) error {
	fs := flag.NewFlagSet("perkey", flag.ExitOnError)
	all := fs.String("all", "", "color base r,g,b para todas las teclas")
	brightness := fs.Int("brightness", 5, "brillo 0-5")
	apply := fs.Bool("apply", false, "aplica lo guardado")
	_ = fs.Parse(args)

	st, err := store.Load()
	if err != nil {
		return err
	}
	if *apply {
		if err := svc.ApplyPerKey(st.PerKey); err != nil {
			return err
		}
		fmt.Println("OK")
		return nil
	}

	colors := map[string]state.RGB{}
	if *all != "" {
		var r, g, b uint8
		if _, err := fmt.Sscanf(*all, "%d,%d,%d", &r, &g, &b); err != nil {
			return fmt.Errorf("color inválido %q", *all)
		}
		for name := range aula.KeyNameToIndex {
			colors[name] = state.RGB{r, g, b}
		}
	}
	for _, arg := range fs.Args() {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("formato esperado tecla=r,g,b, recibí %q", arg)
		}
		var r, g, b uint8
		if _, err := fmt.Sscanf(parts[1], "%d,%d,%d", &r, &g, &b); err != nil {
			return fmt.Errorf("color inválido en %q", arg)
		}
		colors[strings.ToLower(parts[0])] = state.RGB{r, g, b}
	}
	if len(colors) == 0 {
		return fmt.Errorf("indicá --all r,g,b o tecla=r,g,b")
	}
	pk := state.PerKey{Brightness: uint8(*brightness), Colors: colors}
	if err := svc.ApplyPerKey(pk); err != nil {
		return err
	}
	st.PerKey = pk
	if err := store.Save(st); err != nil {
		return err
	}
	fmt.Printf("OK (%d teclas)\n", len(colors))
	return nil
}

func cmdRemap(svc *device.Service, store *state.Store, args []string) error {
	fs := flag.NewFlagSet("remap", flag.ExitOnError)
	layer := fs.String("layer", "normal", "normal o fn")
	reset := fs.Bool("reset", false, "borra la capa")
	_ = fs.Parse(args)

	st, err := store.Load()
	if err != nil {
		return err
	}
	if *reset {
		if err := svc.ResetRemap(*layer); err != nil {
			return err
		}
		if strings.EqualFold(*layer, "fn") {
			st.Remap.FN = map[string]string{}
		} else {
			st.Remap.Normal = map[string]string{}
		}
		return store.Save(st)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("indicá pares origen=destino (ej: f1=f1, capslock=lctrl)")
	}
	entries := map[string]string{}
	if strings.EqualFold(*layer, "fn") {
		for k, v := range st.Remap.FN {
			entries[k] = v
		}
	} else {
		for k, v := range st.Remap.Normal {
			entries[k] = v
		}
	}
	for _, arg := range fs.Args() {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("formato esperado origen=destino, recibí %q", arg)
		}
		entries[strings.ToLower(parts[0])] = parts[1]
	}
	if err := svc.ApplyRemap(*layer, entries); err != nil {
		return err
	}
	if strings.EqualFold(*layer, "fn") {
		st.Remap.FN = entries
	} else {
		st.Remap.Normal = entries
	}
	if err := store.Save(st); err != nil {
		return err
	}
	fmt.Printf("OK (%d remapeos)\n", len(entries))
	return nil
}

func cmdLCD(svc *device.Service, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usá: lcd color r g b | lcd image archivo.gif [--yes]")
	}
	switch args[0] {
	case "color":
		if len(args) != 4 {
			return fmt.Errorf("usá: lcd color r g b")
		}
		var rgb [3]uint8
		for i := 0; i < 3; i++ {
			v, err := strconv.Atoi(args[i+1])
			if err != nil || v < 0 || v > 255 {
				return fmt.Errorf("componente inválido %q", args[i+1])
			}
			rgb[i] = uint8(v)
		}
		if err := svc.LCDSolid(rgb[0], rgb[1], rgb[2]); err != nil {
			return err
		}
		fmt.Println("OK")
		return nil
	case "image":
		// Accept --yes before or after the file name.
		file := ""
		yes := false
		startChunk := 0
		rest := args[1:]
		for i := 0; i < len(rest); i++ {
			switch {
			case rest[i] == "--yes" || rest[i] == "-yes":
				yes = true
			case rest[i] == "--start" && i+1 < len(rest):
				i++
				startChunk, _ = strconv.Atoi(rest[i])
			case strings.HasPrefix(rest[i], "--start="):
				startChunk, _ = strconv.Atoi(strings.TrimPrefix(rest[i], "--start="))
			default:
				file = rest[i]
			}
		}
		if file == "" {
			return fmt.Errorf("usá: lcd image archivo.gif [--yes]")
		}
		if !yes {
			fmt.Fprintln(os.Stderr, "Aviso: subir imágenes puede dañar los gráficos internos del teclado.")
			fmt.Fprintln(os.Stderr, "Volvé a ejecutar con --yes para confirmar.")
			return nil
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		progress := func(sent, total int) {
			fmt.Fprintf(os.Stderr, "\rsubiendo %d/%d páginas", sent, total)
		}
		if err := svc.LCDImageFrom(f, startChunk, progress); err != nil {
			return err
		}
		fmt.Println("\nOK")
		return nil
	default:
		return fmt.Errorf("subcomando lcd desconocido %q", args[0])
	}
}

func cmdKeys() error {
	fmt.Println("Teclas disponibles:")
	names := make([]string, 0, len(aula.KeyNameToIndex))
	for n := range aula.KeyNameToIndex {
		names = append(names, n)
	}
	sortStrings(names)
	for i, n := range names {
		if i%6 == 0 && i > 0 {
			fmt.Println()
		}
		fmt.Printf("%-14s", n)
	}
	fmt.Println()
	return nil
}

func cmdModes() error {
	for i := 0; i <= 19; i++ {
		fmt.Printf("  %2d  %s\n", i, aula.ModeNames[aula.LightingMode(i)])
	}
	return nil
}

func cmdServe(svc *device.Service, store *state.Store, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8723", "dirección de escucha")
	idle := fs.Duration("idle", 0, "apagarse tras N de inactividad (0 = nunca)")
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	srv := api.New(svc, store)
	if *idle > 0 {
		srv.SetIdleShutdown(*idle, cancel)
	}
	fmt.Fprintf(os.Stderr, "openaula API en http://%s\n", *addr)
	return srv.Run(ctx, *addr)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

var _ = time.Second
