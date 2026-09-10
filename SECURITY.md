# Seguridad

**Estado: versión técnica alpha, no auditada externamente.** No uses esta versión para redes sensibles ni con personas en las que no confíes. El objetivo es jugar con amigos, no proporcionar anonimato, aislamiento de aplicaciones o acceso corporativo.

## Reportar un problema

Usa [Report a vulnerability](https://github.com/EspacioKoop/elciber/security/advisories/new) si GitHub ofrece el canal privado en este repositorio. Si no está disponible, abre un issue que solicite un canal privado **sin** incluir la vulnerabilidad, invitaciones, direcciones ni registros. No hay SLA de respuesta ni recompensa prometida.

## Fronteras

- La interfaz y la API son locales; el servidor escucha solo en loopback. **No lo publiques mediante proxy inverso ni port forwarding.**
- Las salas e invitaciones contienen material de acceso. Una invitación no es un enlace de un solo uso, no caduca y no es cifrado de almacenamiento: quien la tenga puede entrar en esa red.
- El guardado local de secretos no sustituye el cifrado del disco ni el control de acceso del sistema. En Windows, el perfil de usuario y sus ACL importan; los modos Unix no acreditan aislamiento NTFS.
- Se arranca en inspección: sin adaptador VPN. El modo VPN necesita una opción explícita de arranque y puede necesitar privilegios del sistema. En esta alpha, elevar el proceso también eleva su controlador local: **separar el helper privilegiado sigue pendiente**.
- El motor se ejecuta como proceso externo. La API no admite ejecutables, shell, argumentos libres, rutas de red, proxy doméstico, exit nodes ni configuración de DNS.
- EasyTier 2.6.4 proporciona el transporte. El cliente habilita su modo seguro; para VPN exige la clave pública del nodo inicial. EasyTier 2.6.4 prioriza una prueba válida del secreto compartido sobre el pin: otro miembro de la sala puede ser aceptado con una clave distinta. No se promete pin estricto frente a miembros. Un hash de descarga no es una auditoría del protocolo.
- El nodo de encuentro y cualquier relay pueden observar metadatos de conexión. No hay garantía de anonimato ni de disponibilidad del nodo.
- Entrar en una VPN permite alcanzar servicios del equipo según sus reglas de firewall. **Una sala no es un sandbox por juego.** No desactives el firewall ni compartas toda tu subred.

## Controles y límites

Consulta [modelo de amenazas](docs/THREAT-MODEL.md) para el contraste entre garantías, implementación y límites. Si un cambio altera autenticación, validación, persistencia, privilegios, configuración del motor o red accesible, necesita actualizar ese documento y añadir regresiones.

No afirmamos auditoría criptográfica, protección frente a malware local, revocación individual, estabilidad prolongada, seguridad del controlador ni compatibilidad universal con juegos.
