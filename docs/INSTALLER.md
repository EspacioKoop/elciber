# Instalador único · candidato técnico

**Objetivo:** instalar El Ciber, abrirlo y jugar sin instalar motores ni configurar nodos a mano. **Todavía no está cumplido.**

## Implementado y comprobado

- Instalador Windows NSIS real, compilado y extraído para verificar su contenido.
- Cliente gráfico sin consola, acceso en Inicio y desinstalador.
- Descarga automática de EasyTier 2.6.4 durante la instalación: el usuario no ejecuta scripts ni necesita Go, Python o PowerShell.
- Helper con HTTPS, tiempo/tamaño acotados, SHA-256 fijado, extracción limitada, manifest por archivo y rechazo de instalaciones existentes.
- Descarga real del paquete Windows y comprobación de integridad ejecutadas desde Linux. Eso **no demuestra una instalación en Windows**.
- Cierre desde la propia interfaz, autenticado y probado contra el proceso real. Las salas se conservan.

Es un instalador web: necesita Internet y descarga los binarios desde el upstream durante la instalación. El EXE del instalador no redistribuye los binarios del motor ni de Wintun.

## Pendiente antes de entregarlo a jugadores

1. Validar encuentro mediante nodos comunitarios gratuitos compatibles con el modo seguro y claves públicas fiables; no alquilar un servidor.
2. Integrar encuentro automático, P2P preferente y relay gratuito de respaldo; retirar la configuración técnica del flujo normal.
3. Instalar/desinstalar en Windows 11 real; verificar UAC, antivirus, adaptador y ausencia de residuos.
4. Partida real entre equipos en redes distintas.

No se ha seleccionado una infraestructura comunitaria compatible ni desplegado un servicio propio. No se ha contratado infraestructura ni expuesto ningún servidor preexistente. La decisión vigente es utilizar encuentro gratuito, sin servidores de pago; véase [estado del proyecto](../PROJECT_STATUS.md). El binario no está firmado ni publicado como release.

## Privilegios y límites actuales

El instalador y esta versión del cliente solicitan permisos de administrador. Aún no se ha separado un helper privilegiado: **no ejecutar esta alpha en un equipo sensible**. La solicitud UAC del cliente se repite al abrirlo; no es una elevación silenciosa.

No se desactiva el firewall, no se configura UPnP, no se añade autoarranque ni un servicio permanente. El desinstalador conserva los datos del perfil. Se rechaza actualizar sobre una instalación existente; reparación, rollback tras corte abrupto y actualización automática siguen pendientes.

## Construcción reproducible

Requiere Go 1.27.1, NSIS 3 y `github.com/akavel/rsrc` v0.10.2 como herramienta de compilación. Desde un entorno Linux con esas herramientas en PATH:

```bash
python3 scripts/build_windows_installer.py
```

El script usa un árbol temporal, genera el recurso Windows y compila el cliente y el descargador. No ejecuta el instalador, no instala controladores y no modifica el sistema. Genera un EXE y un fichero SHA-256 bajo `dist/`. El checksum no es una firma digital.

Ingeniería: **OTACON Astra**.
