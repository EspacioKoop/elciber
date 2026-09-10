# Estado del proyecto y punto de reanudación

**Actualizado: 2026-09-10 · Versión de desarrollo: 0.1.0-alpha.1.**

Este es el punto de entrada para retomar El Ciber desde un clon nuevo, sin depender del historial de un chat. Describe implementación, decisiones y pendientes por separado. El commit de la rama publicada identifica el candidato exacto; la fecha de este documento no acredita pruebas posteriores.

## Espacio de colaboración

Repositorio canónico: **[EspacioKoop/elciber](https://github.com/EspacioKoop/elciber)**. El proyecto se transfirió a la organización compartida el 2026-09-10, conservando identidad e historial. `VaroTv7` y `eGurucharri` tienen acceso para colaborar; las autorizaciones actuales deben consultarse en GitHub antes de cambiar permisos.

El Ciber sigue siendo independiente de los demás proyectos de la organización. No se copian sus datos, contratos o infraestructura. Los agentes no reciben autorización de trabajo autónomo por este traslado. Antes de editar, revisar `main`, estado local, issues y PR abiertos; usar ramas propias y PR para no pisar trabajo paralelo.

## Objetivo acordado

Una aplicación **gratuita y open source** para jugar por LAN virtual con amigos, con salas e invitaciones: **instalar → crear sala o aceptar invitación → jugar**.

- Windows 11 x64 primero; Linux es por ahora el entorno de validación técnica.
- Un instalador prepara el cliente y el motor; no instalar EasyTier por separado ni pedir Go, Python, consola, nodos, IPs o claves técnicas al jugador.
- **Sin servidores de pago como requisito del proyecto o de los jugadores.** No hay autorización para contratar infraestructura.
- **P2P directo como primera opción.** El encuentro gratuito sirve para descubrir pares; una vez establecida una conexión directa, el tráfico de juego circula entre ellos.
- **Relay comunitario gratuito solo como respaldo** cuando no pueda establecerse el enlace directo. Su disponibilidad, ubicación y capacidad pueden afectar al rendimiento; no se garantiza conectividad directa universal ni ausencia de lag.
- No prometer más velocidad o estabilidad que Hamachi/ZeroTier sin comparación real. Medir consumo del cliente, motor y navegador, no solo tamaño del instalador.
- No desplegar servicios en redes preexistentes, abrir puertos ni contratar un servidor para resolver decisiones pendientes de esta alpha.

La infraestructura comunitaria está documentada por [EasyTier](https://www.easytier.cn/en/guide/network/quick-networking.html). **Todavía no se ha seleccionado ni validado un conjunto de nodos para El Ciber.** La existencia de nodos gratuitos no acredita compatibilidad con el modo seguro, autenticación, latencia ni condiciones de uso de cada uno.

## Qué existe realmente

- Backend Go y frontend es-ES embebido, sin dependencias web de ejecución ni Electron.
- Salas locales persistentes, invitaciones versionadas e importación sin autoconexión.
- API loopback con sesión efímera, comprobación de origen/Host y validación de entradas.
- Supervisión real de EasyTier 2.6.4, configuración privada, estado/presencia reales y cierre del proceso propio.
- Inspección sin TUN y capacidad VPN explícita. **P2P automático, STUN y hole punching están desactivados en el código actual, también en VPN.** El diseño acordado aún debe implementarse.
- Candidato de instalador web Windows compilado y extraído: descarga automática del motor, hashes fijados, acceso de Inicio y desinstalador. No depende de Python/Go/PowerShell en el equipo del jugador.
- Cierre desde la interfaz, sin necesidad de consola, con conservación de las salas.
- README con capturas reales, licencia MIT del cliente, avisos de terceros, pruebas y documentación. Configuración CI Windows/Linux preservada como plantilla; no se activa automatización al publicar este snapshot.

## Evidencia y límites

Los detalles están en [VALIDATION.md](docs/VALIDATION.md) y [INSTALLER.md](docs/INSTALLER.md).

- Pruebas Go con detección de carreras, análisis estático y compilaciones Linux/Windows ejecutadas.
- Navegador real y motor oficial real sobre loopback: invitación, handshake, presencia, desconexión y cierre. No TUN ni cambios de interfaces/rutas en esas pruebas.
- Helper del instalador probado con archivos sintéticos y descarga real del paquete Windows desde Linux, verificando el manifest y sus hashes.
- El instalador se ha compilado y extraído; **no se ha instalado ni ejecutado en Windows real**. No está firmado ni hay una release de jugador validada.
- No se han probado partidas entre redes distintas, compatibilidad LAN por juego, consumo prolongado ni rendimiento P2P/relay.
- La configuración de GitHub Actions se conserva como [plantilla sin activar](docs/ci/README.md). No hay CI remota aprobada asociada a esta publicación; las pruebas acreditadas son locales.

**Resultado actual: base técnica recuperable; no producto listo para instalar y jugar.**

## Trabajo pendiente, en orden

1. **Encuentro gratuito compatible y autenticado.** Evaluar nodos comunitarios, obtención fiable de claves públicas, condiciones de uso y fallo de nodos. No quitar validaciones de seguridad para aceptar cualquier servidor.
2. **P2P preferente y respaldo relay.** Integrar configuración automática, NAT traversal y selección de rutas con estado real visible. Mantener las pruebas de inspección aisladas, sin habilitar tráfico externo en ellas.
3. **Flujo sin configuración técnica.** Crear/importar sala debe preparar el motor y la conexión sin que el jugador introduzca direcciones de nodos, claves o parámetros de red. Los formularios técnicos actuales no satisfacen este criterio.
4. **Endurecer el instalador y privilegios.** Separar la interfaz del helper privilegiado; reparar/reintentar tras fallo, preservar datos y ofrecer actualización/rollback. El candidato actual solicita UAC al abrir el cliente y rechaza instalaciones existentes.
5. **Validación Windows 11.** Instalación, apertura, adaptador, desconexión, desinstalación y conservación de datos en un equipo autorizado de pruebas. No ejecutarlo en un servidor de producción.
6. **Partida entre dos redes.** Probar juegos concretos, conexión directa y relay por separado, estabilidad, pérdida de paquetes, jitter, consumo y recuperación tras suspensión/corte. Registrar resultados reales, no compatibilidad inferida de un ping.

No sustituir estos criterios por nuevas funciones de chat, voz o personalización. No hay promesa de fecha ni autorización de crons/desarrollo autónomo asociada a este documento.

## Límites que deben conservarse al retomar

- Una invitación contiene acceso compartido: no publicarla, no caduca y eliminar una sala local no revoca las copias ajenas.
- EasyTier 2.6.4 puede aceptar una prueba válida del secreto de sala por encima de un pin diferente. El pin no es autenticación exclusiva frente a miembros que ya conocen ese secreto.
- El TOML de modo seguro requiere un par X25519 real; activar solo la opción puede dar RPC saludable sin handshake válido.
- No imprimir la respuesta completa de `node info`: puede incluir configuración con secretos.
- El helper de instalación no ejecuta los binarios descargados. Rechaza destinos existentes; un corte abrupto puede dejar un destino parcial que requiera recuperación todavía no implementada. `0700` no equivale a una ACL Windows.
- Un TUN no reproduce todo Ethernet. El descubrimiento automático y broadcast deben comprobarse por juego.

## Mapa para reanudar

- [README](README.md): presentación, capturas y arranque de desarrollo.
- [ROADMAP](ROADMAP.md): prioridades y criterios de aceptación.
- [Arquitectura](docs/ARCHITECTURE.md) y [API](docs/API.md): implementación actual.
- [Conexión](docs/CONNECTING.md): procedimiento técnico de la alpha, no experiencia final.
- [Seguridad](SECURITY.md), [amenazas](docs/THREAT-MODEL.md) y [terceros](THIRD_PARTY_NOTICES.md).
- `main.go`, `api.go`, `engine.go`, `engine_config.go`, `room.go`, `store.go`: cliente/controlador.
- `web/`: interfaz. `cmd/setup-engine/`, `packaging/windows/` y `scripts/build_windows_installer.py`: instalador.
- `tests/`, pruebas Go y `scripts/smoke_engine.py`: regresiones y pruebas reales acotadas.

## Política de continuidad

Cada hito y cierre de trabajo debe dejar código, decisiones, pruebas y pendientes coherentes en Git. Publicar el commit y comprobar la rama remota antes de afirmar que el trabajo está respaldado. Un fallo de publicación debe quedar explícito y conservar el candidato local; no ampliar permisos ni eludir controles unilateralmente.

GitHub conserva **lo publicado**, no cambios aún sin commit/push. No es sincronización instantánea ni copia de los datos privados de las salas. Los binarios, motores descargados, cachés, invitaciones y perfiles locales no se suben: los artefactos de desarrollo se reconstruyen desde sus fuentes y versiones fijadas. No se adjunta el chat ni información de infraestructura personal.

Cuando cambie un criterio, actualizar este archivo junto con README/ROADMAP y la evidencia afectada; no presentar una aspiración como implementación. La continuidad se mantiene durante el trabajo autorizado, no mediante tareas persistentes no solicitadas.
