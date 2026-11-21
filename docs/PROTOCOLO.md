# Protocolo del AULA / Epomaker F108 Pro

Resumen de lo que implementa openaula. Hay dos transportes distintos.

Créditos: el camino cableado lo reversó Parsia
([parsiya/f108-pro](https://github.com/parsiya/f108-pro), MIT). El camino 2.4 GHz
está documentado por kidush
([omarchy-aula-f108pro](https://github.com/kidush/omarchy-aula-f108pro), MIT) a
partir del driver oficial de Windows.

## Cable USB-C — `0c45:800a`

Interfaz 3 (usage page `0xFF13`), feature reports de 64 bytes, sin report ID.
Se accede con `HIDIOCSFEATURE` / `HIDIOCGFEATURE` (o control transfers
equivalentes), con 35 ms entre paquetes.

Transacción: `04 18` (begin, con lectura de ACK) → `04 xx` (init) → datos →
`04 02` (apply) → `04 F0` (finalize). La lectura de ACK es obligatoria: sin ella
el teclado ignora lo que sigue.

| Comando | Uso |
| --- | --- |
| `04 13` | Luz de fondo: modo, RGB, colorful, brillo, velocidad, dirección |
| `04 23` | Per-key: 576 bytes, entrada en `key_index*4` = `índice R G B` |
| `04 11` / `04 27` | Remapeo capa normal / FN: tabla de 576 bytes en 9 reportes |
| `04 28` | Reloj del LCD |
| `04 72` | Cabecera de subida de imagen al LCD |
| interfaz 2 | Páginas de 4096 bytes con los píxeles RGB565 |

La tabla de remapeo lleva `AA 55` en los últimos dos bytes. El bloque de luz usa
`AA 55` en los offsets 14-15 de su payload.

## Dongle 2.4 GHz — `05ac:024f`

Interfaz 3 (usage page `0xFF60`, usage `0x61`), 32 bytes de salida y 32 de
entrada, sin report ID. Se escribe `0x00` + 32 bytes (el 0x00 es el report ID) y
se espera un reporte de entrada que repita los primeros tres bytes.

Cada paquete es autocontenido: no hay begin/apply ni trailers `AA 55` generales.
El último byte (offset 31) es el checksum: suma de los bytes 0..30 módulo 256.

```
off 0     cmd
off 1     subcomando / longitud de bloque
off 2     índice de chunk
off 3..30 datos (máx. 28 bytes)
off 31    checksum
```

| Packet | Uso |
| --- | --- |
| `02 00 00` | Probe: ¿el teclado está despierto detrás del dongle? |
| `20 01 00` | Batería: el porcentaje viene en el byte 3 de la respuesta |
| `05 10 00` | Luz de fondo (mismo bloque de 16 bytes que el cable, desplazado 3) |
| `0C 10 00` | Reloj del LCD |
| `10 1C <chunk>` / `12 1C <chunk>` | Tabla de remapeo normal / FN, 21 chunks de 28 bytes |
| `14 1C <chunk>` | Tabla per-key, precedida por `05 10` con modo `0x80` |
| `03 04 <idx>` / `0D 04 <idx>` | Remapeo de una tecla, normal / FN |
| `1F 00 00` | Reset de mapeo |
| `7F 03 <slot> <chunks>` | Cabecera de subida de imagen al LCD (experimental) |
| `80|i>>16`, `i&0xFF`, `i>>8` | Chunks de la imagen, 28 bytes cada uno |

Extras que expone el firmware: video/notificaciones en la colección de usage
page `0xFFFF` (no presente en este dongle), y ajustes de función `07 10`
(deshabilitar Alt+Tab, Alt+F4, tecla Windows, `fn_switch`, tiempos).

## Qué no se puede

- Leer el estado actual: la configuración es write-only en ambos transportes.
- Setear el modo de sistema (Windows/Mac/iOS/Android) por HID: lo decide el
  firmware con FN+Q/W/E/R.
- Controlar la barra de luz y las tiras laterales: solo por combos FN.
- La versión de firmware se deduce de `bcdDevice` (`0x0138` → `138`), no hay
  comando de versión.
