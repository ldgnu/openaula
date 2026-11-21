# openaula

Configuración del teclado **AULA / Epomaker F108 Pro** en Linux: luz de fondo,
color por tecla, remapeo de teclas (capa normal y FN), reloj del LCD, imágenes
en el LCD y perfiles guardables.

Es un solo binario en Go con tres formas de uso:

- **TUI** (por defecto): menús con flechas y Enter.
- **CLI**: un comando por acción, para scripts.
- **API HTTP/JSON** opcional: solo si corrés `openaula serve`.

No hay proceso residente. Cada acción abre el teclado, manda el comando y sale.
Cuando no hay nada corriendo, openaula no consume CPU, memoria ni USB.

## Estado del soporte

| Función | USB-C (`0c45:800a`) | Dongle 2.4 GHz (`05ac:024f`) |
| --- | --- | --- |
| Detección | sí | sí |
| Luz de fondo | sí | **no** |
| Color por tecla | sí | **no** |
| Remapeo | sí | **no** |
| Reloj del LCD | sí | **no** |
| Imagen en el LCD | sí | **no** |

La configuración se hace **solo por cable USB-C**. El canal vendor del dongle
2.4 GHz no está documentado y, durante el desarrollo, mandarle paquetes colgó el
receptor hasta desconectarlo y volver a conectarlo. openaula detecta el dongle
pero no escribe en él.

Lo que configurás por cable queda guardado en el firmware del teclado: después
lo desconectás y seguís usando 2.4 GHz o Bluetooth con la misma configuración.

## Requisitos

- Go 1.24 o superior.
- libusb (cgo). En Gentoo: `emerge dev-libs/libusb`. En Debian/Ubuntu:
  `apt install libusb-1.0-0-dev pkg-config`.

## Compilar e instalar

```bash
make build          # genera ./openaula
sudo make install   # copia a /usr/local/bin
sudo make udev      # regla udev para no necesitar root
```

La regla udev da acceso al grupo `usb` con modo `0660`. Verificá que tu usuario
esté en ese grupo (`id -nG`).

## Uso

```bash
openaula            # TUI
openaula status     # estado del dispositivo
```

### TUI

Flechas para mover, izquierda/derecha para cambiar un valor, Enter para actuar,
Esc para volver. Cada pantalla muestra sus teclas abajo, sin atajos ocultos.

Menú:

- **Luz de fondo**: 20 efectos, color RGB, brillo, velocidad y dirección.
- **Color por tecla**: editor sobre la disposición ANSI de 108 teclas.
- **Remapear teclas**: capa normal o FN, con búsqueda por nombre.
- **Ajustes rápidos**: F1–F12 directos, restaurar Enter.
- **Sistema operativo**: Windows/Linux, macOS, iPadOS/iOS, Android.
- **Reloj del LCD**, **Pantalla del LCD**, **Perfiles**, **Estado**.

### CLI

```bash
openaula light --mode 1 --color 255,0,0 --brightness 5   # rojo estático
openaula light --mode 7 --color 0,0,255 --speed 3       # respirar azul
openaula off                                            # apagar
openaula preset fkeys                                   # F1..F12 sin Fn
openaula preset enter                                   # restaurar Intro
openaula os macos                                       # modificadores para macOS
openaula perkey --all 0,0,40 w=0,255,0 a=0,255,0        # WASD verde
openaula remap capslock=lctrl f1=media:play --layer normal
openaula clock                                          # reloj del LCD
openaula lcd color 255,0,0                              # LCD rojo
openaula lcd image logo.png --yes                       # PNG, GIF o JPEG
```

### API

Es opt-in y se puede apagar sola por inactividad:

```bash
openaula serve --addr 127.0.0.1:8723 --idle 60s
```

```bash
curl -s localhost:8723/api/v1/status
curl -s -X POST localhost:8723/api/v1/light \
  -d '{"mode":1,"r":255,"g":0,"b":0,"brightness":5}'
curl -s -X POST localhost:8723/api/v1/presets/fkeys
curl -s -X POST localhost:8723/api/v1/os/macos
curl -s -X POST localhost:8723/api/v1/clock
curl -s -X POST -F file=@logo.png -F confirm=true localhost:8723/api/v1/lcd/image
```

Endpoints: `GET /api/v1/status`, `/modes`, `/keys`, `/state`, `/profiles`,
`/health`; `PUT /state`; `POST /light`, `/off`, `/perkey`, `/remap`,
`/remap/reset`, `/clock`, `/presets/{id}`, `/os/{id}`, `/lcd/color`,
`/lcd/image`, `/profiles/{name}`, `/profiles/{name}/apply`;
`DELETE /profiles/{name}`.

## F1–F12 sin Fn

El firmware arranca con la fila F en modo multimedia: F1..F12 mandan
volumen/play/etc. y las teclas de función quedan detrás de Fn. El preset
`fkeys` remapea la capa normal para que F1..F12 manden sus códigos HID reales;
los atajos multimedia siguen en Fn+F1..F12.

## Teclas que no responden

Si Intro no hace nada, probá `openaula preset enter`, que fuerza Enter
(HID `0x28`) en la capa normal. Si tampoco así responde, el problema es del
firmware o del switch, no del remapeo.

## Lo que el protocolo no permite

- No se puede **leer** la configuración actual del teclado: luz, colores por
  tecla, remapeos y versión de firmware no son consultables. openaula guarda lo
  último aplicado en `~/.config/openaula/state.json` y lo usa como referencia.
- No se puede poner el teclado en modo Mac/Windows/iOS/Android por software: eso
  lo decide el firmware con FN+Q/W/E/R. Los perfiles de sistema de openaula
  compensan los modificadores por remapeo.
- El control del LCD no expone el brillo ni el contraste de la pantalla.
- La barra de luz superior y las tiras laterales solo se manejan con combos FN
  del teclado; el firmware acepta comandos pero no los aplica.

## Cuidado con el LCD

Subir imágenes puede dañar de forma permanente los gráficos internos del menú
del teclado. openaula bloquea GIFs de más de 141 cuadros y pide confirmación
explícita (`--yes` o `confirm=true`). Los colores sólidos usan el mismo
generador y son la opción segura.

El LCD es de 240x135. Las imágenes estáticas se escalan manteniendo proporción
y se centran sobre fondo negro.

## Créditos y licencia

El protocolo HID fue reversado por Parsia en
[parsiya/f108-pro](https://github.com/parsiya/f108-pro) (MIT). `internal/aula`
deriva de ese trabajo; ver `NOTICE.md`. Licencia MIT.
