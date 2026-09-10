# Evidencia de validación · 0.1.0-alpha.1

**Fecha: 2026-09-10.** Estas pruebas acreditan una base de desarrollo, no un instalador terminado ni una VPN de juego validada entre viviendas.

## Ejecutado localmente

- **Go 1.27.1 / Linux x64:** suite completa con `-race -count=1`, incluyendo pruebas opt-in con EasyTier oficial 2.6.4, aprobada. `go vet` aprobado.
- **Compilación:** ejecutables Linux/amd64 y Windows/amd64 generados con `-trimpath`. Compilar Windows desde Linux no demuestra ejecución en Windows.
- **Navegador Chromium real:** siete pruebas aprobadas contra dos instancias Go aisladas. Estados vacíos, cero solicitudes externas de la UI, crear/persistir, exportar/importar una invitación real sin autoconectar, render seguro de nombres, eliminación explícita, error de importación, teclado y formulario a 390 px.
- **Continuidad de formulario:** texto y foco sobreviven al intervalo de refresco. Se corrigió la selección de una sala recién importada cuando terminaba una consulta antigua de estado.
- **Descargador:** descarga real del paquete Linux fijado, SHA-256 verificado y `easytier-core --version` ejecutado. Cuatro regresiones unitarias con archivos expresamente sintéticos verifican rechazo de checksum, conservación de instalación existente, selección de archivos y limpieza de staging.
- **Integración real por HTTP:** sala creada → invitación real → motor EasyTier → presencia de un peer de referencia → desconexión y retirada de configuración temporal. Comparación de interfaces y rutas antes/después: sin cambios. No se ha creado TUN ni conectado a nodos públicos.
- **Autenticación real del motor:** handshake Noise/X25519 con claves reales, presencia de otro peer y prueba de la semántica del pin. El nodo extranjero se acepta con pin correcto y se rechaza con pin incorrecto. Un nodo con el secreto de sala se acepta pese a un pin distinto: es una limitación upstream explícita, no una garantía de pin estricto.
- **Capturas:** `docs/images/01-inicio.png`, `02-sala.png` y `03-movil.png` proceden del código real. Nombres y juego son datos locales de prueba; no se muestran invitaciones ni una partida simulada. El banner es una ilustración original, no evidencia de runtime.

## Candidato de instalador y cierre gráfico · comprobación posterior

- Instalador NSIS compilado y extraído. Cliente y helper Windows/amd64 comprobados como ejecutables del subsistema gráfico; no se ejecutaron en Windows.
- Helper de instalación probado con detección de carreras, casos offline de ZIP/TLS/límites/limpieza y conservación de destinos existentes. Descarga real del paquete Windows desde Linux, manifest y hashes por archivo comprobados.
- Cierre autenticado por API y desde la interfaz añadido. El smoke contra el proceso real comprueba cierre normal y limpieza; suite Go y navegador aprobadas localmente.
- Todavía no hay instalación/desinstalación Windows real, actualización/recuperación tras corte abrupto, encuentro automático, partida entre redes ni mediciones de rendimiento.

## Qué se corrigió durante las pruebas

1. Activar `[secure_mode] enabled=true` en TOML no generaba por sí solo las claves necesarias. El cliente ahora genera un par X25519 mediante la biblioteca estándar de Go y lo entrega únicamente en el archivo privado; el peer del harness usa la inicialización real de EasyTier.
2. Una respuesta de estado antigua podía borrar la selección de una sala recién guardada. La UI obtiene el estado posterior al guardado antes de aplicar la selección.
3. La documentación no explicaba la precedencia del secreto de sala sobre el pin. Se ha limitado la promesa en API, guía, modelo de amenazas y confirmación, con regresión real.

## Reproducir

```bash
go test -race -count=1 ./...
go vet ./...
go build -trimpath -o dist/elciber .
npm ci
npx playwright install chromium
npm test
python3 -m unittest discover -s tests -p 'test_*.py'
python3 scripts/fetch_engine.py # únicamente si el directorio aún no existe
ELCIBER_TEST_ENGINE_DIR="$PWD/engines/easytier" go test -race -count=1 ./...
python3 scripts/smoke_engine.py
```

Las pruebas opt-in con el motor se omiten si no se define `ELCIBER_TEST_ENGINE_DIR`. Los tests de navegador usan puertos loopback dedicados, datos temporales y un motor deliberadamente ausente. El smoke usa un motor real sin TUN. Los tres niveles son diferentes y no deben confundirse.

## Pendientes que impiden llamarlo producto listo

- Instalación única con motor incluido y experiencia sin configuración técnica manual.
- Ejecución del cliente, descargador y adaptador en Windows 11 real.
- Dos equipos en redes físicas distintas; tráfico TCP/UDP de juego y partida sostenida.
- Descubrimiento LAN/broadcast por juego, NAT restrictivo, reconexión, suspensión, crash y retirada del adaptador.
- Medidas de rendimiento con hardware, carga, duración y método publicados.
- Helper privilegiado separado, credenciales revocables y revisión de distribución conjunta de dependencias.

La configuración de GitHub Actions se conserva como [plantilla sin activar](ci/README.md). No hay ejecuciones aprobadas de CI asociadas a esta primera publicación; la evidencia anterior procede de pruebas locales. No hay despliegue público de nodos, servicios persistentes ni pruebas en equipos privados de amigos asociadas a esta publicación.

Ingeniería y validación: **OTACON Astra**.
