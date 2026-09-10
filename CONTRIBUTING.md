# Contribuir a El Ciber

Queremos que entrar a una partida LAN con amigos sea sencillo, no convertir el cliente en un panel de administración de redes.

## Preparar el entorno

1. Instala Go 1.27.1 o compatible y clona el repositorio.
2. Ejecuta `go test -race ./...` y `go vet ./...`.
3. Compila con `go build -trimpath -o dist/elciber .`.
4. Arranca sin privilegios, con un directorio de datos de prueba: `dist/elciber --no-browser --data-dir ./data`.
5. Para la interfaz: Node.js 22+, `npm ci`, `npx playwright install chromium`, `npm test`.

Node y Playwright son herramientas de pruebas, **no dependencias del usuario final**. Para pruebas con el motor: consulta [validación](docs/VALIDATION.md). Nunca uses una sala personal ni credenciales reales en fixtures, trazas o capturas.

## Antes de abrir una PR

- Explica el problema, el cambio visible y cómo reproducir la prueba.
- Añade regresiones para validación, persistencia, autenticación o ciclo de vida que hayas cambiado.
- En UI, comprueba teclado, 390 px, 1440 px, estados vacíos/error y escritura durante polling.
- Conserva el contrato de [API local](docs/API.md) y actualiza seguridad si cambia una garantía.
- No añadas telemetría, nodos externos predeterminados, autoarranque privilegiado, controladores o cambios de firewall como efectos colaterales.
- No subas invitaciones, ficheros de sala, rutas domésticas, logs de red o datos de terceros.
- No afirmes compatibilidad con un juego basándote solo en una compilación o un ping.

Las PR se revisan antes de integrarse. Los cambios de red o privilegios necesitan pruebas específicas y rollback documentado. Una discusión de diseño no demuestra que una funcionalidad esté terminada.

## Continuidad del proyecto

[PROJECT_STATUS.md](PROJECT_STATUS.md) conserva decisiones, implementación, evidencia y siguiente trabajo. Leerlo al retomar y actualizarlo con cada hito o cierre, junto con README/ROADMAP y las pruebas afectadas. No basta guardar código localmente: comprobar el commit remoto antes de afirmar que quedó respaldado.

Los cambios de un hito deben quedar publicados dentro del trabajo autorizado o tener un bloqueo explícito. Git no copia invitaciones, perfiles, motores descargados ni cambios sin publicar, ni garantiza sincronización instantánea antes de un corte eléctrico. No se añaden crons o auto-publicación sin revisión.

## Reportar un juego

Indica juego y versión, sistema, número de equipos, si estaban en redes distintas, conexión directa/relay, conexión por IP o descubrimiento automático y pasos de reproducción. Sustituye IPs públicas, nombres de equipos e invitaciones por marcadores. Usa la plantilla de compatibilidad.

## Seguridad

No publiques detalles explotables ni secretos en issues. Sigue [SECURITY.md](SECURITY.md).
