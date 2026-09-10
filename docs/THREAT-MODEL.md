# Modelo de amenazas · alpha

## Activos y actores

Protegemos el material de acceso a salas, el control local del motor, la integridad del almacenamiento y la decisión de cuándo conectar. Consideramos páginas web maliciosas, invitaciones manipuladas, peers no confiables, errores de disco y procesos de red que fallan.

**No se protege contra malware local, administrador hostil, kernel/controlador comprometido o una persona de confianza que comparta una invitación.** Una sala no aísla servicios por juego.

## Garantías contrastables

| Garantía del cliente | Control vigente | Límite explícito |
|---|---|---|
| La API no escucha en interfaces públicas. | `validateListen`, `net.Listen`, comprobación de dirección remota loopback. | Exponerla mediante un proxy puede romper la frontera; no es un servicio multiusuario. |
| Una página de otro origen no puede invocar la API en condiciones normales de navegador. | Host exacto, Origin exacto cuando existe, Fetch Metadata, sesión efímera y cabecera propia, sin CORS. | SOP y navegador forman parte del modelo; malware local puede leer la página y obtener sesión. |
| La UI no admite scripts externos ni ser embebida. | CSP, `frame-ancestors 'none'`, X-Frame-Options, recursos locales. | No sustituye revisión del JavaScript ni protege extensiones del navegador comprometidas. |
| Importar no arranca la red. | Endpoint de importación solo valida y persiste; `connect` exige confirmación separada. | Una invitación guardada contiene un destino no confiable que el usuario debe verificar. |
| El cliente no acepta shell o parámetros libres por la API. | Configuración estructurada, campos permitidos, ejecución de rutas fijadas al arrancar. | El propietario puede elegir un binario malicioso con `--engine-dir`; el descargador no es una auditoría. |
| Entradas malformadas se rechazan antes de configurar el motor. | Cuerpos acotados, JSON estricto, límites de sala, validación de identificadores, claves y destinos. | No es un filtro universal de destino/SSRF; el modo VPN conecta el destino confirmado por el usuario. |
| Los secretos de sala no forman parte del listado público. | DTO `Room` sin secreto, exportación de invitación solo mediante acción explícita. | El navegador autorizado puede solicitar invitaciones. La respuesta de exportación es sensible. |
| El secreto no se incluye en argumentos del proceso ni salida de diagnóstico. | TOML temporal privado, entorno acotado y stdout/stderr del motor descartados. | Persiste en archivos de usuario sin cifrado de almacenamiento; procesos del mismo usuario pueden leerlos. |
| El guardado fallido no actualiza la memoria como si hubiese funcionado. | Temporal + sincronización + reemplazo, actualización de memoria después del commit. | Durabilidad del directorio depende del sistema; no equivale a backup ni garantiza supervivencia a fallo físico. |
| Una identidad de sala existente no se sobrescribe con otra invitación. | Importación idempotente si coincide; conflicto si difiere material o metadatos. | No existe aún edición/rotación o revocación individual. |
| La inspección no configura un adaptador virtual. | DHCP desactivado, `no_tun`, sin STUN/UPnP, destinos loopback numéricos y listeners desactivados. | Es un modo técnico sin LAN; la prueba real compara interfaces y rutas antes/después. |
| La VPN no confía únicamente en la dirección del nodo inicial. | EasyTier Secure Mode y `peer_public_key` requerido antes de conectar. | La clave debe obtenerse por canal confiable. EasyTier 2.6.4 acepta también una prueba del secreto de sala aunque el pin difiera: no excluye a un miembro como suplantador del nodo. No afirma auditoría ni anonimato. |
| Desconectar opera sobre el proceso iniciado por este cliente. | Supervisión del hijo, cancelación y limpieza de configuración. | Windows termina el proceso; cierre forzado/crash y retirada del adaptador requieren pruebas propias. |
| Presencia, IP y latencia no son inventadas. | Consulta CLI/RPC del motor y campos sanitizados. | Proceso o RPC operativo no acredita tráfico de juego, conectividad entre casas ni identidad humana. |

## Riesgos que siguen abiertos

1. **Privilegios:** UI y controlador local aún comparten proceso; no ejecutar elevado en un equipo sensible. Falta helper privilegiado acotado.
2. **Almacenamiento:** permisos Unix y ACL del perfil no son cifrado; Windows requiere validación específica. Backups, clipboard y sincronización pueden filtrar invitaciones.
3. **Acceso lateral:** no hay firewall por juego. Un invitado puede alcanzar servicios que permita el sistema.
4. **Red real:** pendientes TUN Windows, NAT/relay entre redes distintas, suspensión, crash y conflictos de subred.
5. **Supply chain:** checksums fijados y versiones delimitan el candidato, no eliminan riesgos de upstream, herramientas de compilación o controladores.
6. **Identidad y expulsión:** acceso compartido sin caducidad ni revocación individual. La función de credenciales temporales del motor no está integrada.
7. **Disponibilidad local:** las peticiones y operaciones se acotan, pero no hay protección frente a un proceso local hostil ni un servicio Internet endurecido con cuotas por usuario.

## Regresiones y revisión

Las pruebas Go cubren validadores, almacenamiento, frontera HTTP y ciclo de vida; Playwright comprueba DOM, formularios e intercambio de invitaciones real; `scripts/smoke_engine.py` usa el motor oficial en loopback. La matriz ejecutada está en [VALIDATION.md](VALIDATION.md).

Cambios de autenticación, rutas API, scopes de escucha, formato de invitación, entorno heredado, escritura en disco, parámetros de motor, privilegios o dependencias requieren actualizar este documento y las regresiones asociadas. Un cambio de UI también necesita verificar que la confirmación muestra el destino real y no pierde el estado durante el refresco.

**No reclamamos certificación, cumplimiento normativo o auditoría de seguridad externa.**
