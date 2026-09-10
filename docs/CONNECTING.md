# De una sala a una partida

## Lo que puedes hacer sin permisos especiales

Esta sección describe el binario de desarrollo, sin el manifest elevado del instalador. Arranca El Ciber sin flags VPN. Puedes crear una sala, darle un nombre y un juego orientativo, generar una invitación y guardarla en otro cliente. Ninguna de esas acciones conecta la red.

El modo **inspección** permite comprobar el proceso EasyTier instalado sin TUN, DHCP, STUN ni destinos externos; solo admite destinos loopback numéricos. Una IP vacía es correcta: **no existe un adaptador virtual para el juego**. Para una prueba automatizada aislada en Linux, usa `python3 scripts/smoke_engine.py`.

## Requisitos de una conexión VPN

La alpha requiere preparación técnica manual. Existe un candidato de instalador compilado, pero el flujo completo sin configuración técnica aún no funciona. El objetivo acordado es encuentro gratuito, P2P preferente y relay de respaldo, sin servidores de pago; véase [estado del proyecto](../PROJECT_STATUS.md).

1. Dos equipos de pruebas, cada uno con El Ciber y **EasyTier core + cli 2.6.4**. En Windows, también el `wintun.dll` del paquete oficial compatible.
2. Un nodo de encuentro EasyTier alcanzable desde ambos equipos. No se despliega ni se paga ningún nodo al usar este repositorio.
3. Modo seguro activado en ese nodo y una clave persistente. Su operador debe facilitarte la **clave pública**, a través de un canal de confianza. El Ciber configura esa clave como pin. En EasyTier 2.6.4, una prueba válida del secreto de sala tiene prioridad: **otro poseedor del secreto puede ser aceptado aunque no coincida el pin**. No es autenticación exclusiva de un dispositivo frente a los propios miembros. La clave privada del nodo **nunca** se pega en El Ciber ni en una invitación.
4. Permisos suficientes para crear el adaptador virtual. El binario de desarrollo no se eleva automáticamente; el candidato empaquetado sí solicita UAC al abrirlo. Todavía no hay helper privilegiado separado: revisa las implicaciones y usa un equipo de pruebas.
5. Una sala nueva cuya dirección y clave pública correspondan a ese nodo. La dirección admite `tcp://host:puerto` o `udp://host:puerto`; no se aceptan credenciales embebidas, comandos ni rutas arbitrarias.

Para configurar el nodo, consulta la [documentación oficial de modo seguro](https://easytier.cn/en/guide/network/secure-mode.html) y el [tag integrado](https://github.com/EasyTier/EasyTier/tree/v2.6.4). Esta guía no presenta un relay sin autenticar como configuración segura. El operador del nodo controla su disponibilidad y puede observar metadatos.

## Secuencia de conexión

1. En un equipo de pruebas, inicia el cliente con `--enable-vpn` y el directorio del motor. La opción solo habilita la capacidad; no conecta al arrancar.
2. Crea la sala con su destino y clave pública. Comprueba ambos con el operador, no solo con un enlace recibido de un desconocido.
3. Comparte la invitación en privado. Los demás la guardan; no se conectarán automáticamente.
4. En cada equipo, revisa la confirmación y conecta. Una sola sala puede estar activa en cada cliente.
5. Espera a que el motor informe una IP virtual y peers reales. Proceso operativo y presencia **no demuestran** que el juego sea alcanzable.
6. Crea una partida en el juego e intenta entrar por la IP virtual del anfitrión si el título lo permite. La lista automática de LAN requiere una prueba distinta.
7. Desconecta en todos los equipos y verifica que se retira la conectividad virtual. Ante un fallo, no desactives el firewall ni abras puertos indiscriminadamente.

La primera versión todavía **no ha superado este ciclo completo con dos Windows 11 en redes distintas**. Es el siguiente criterio de producto.

## Direcciones, firewall y límites

EasyTier 2.6.4 gestiona su DHCP virtual y puede adoptar una subred anunciada por pares; su implementación inicial usa `10.126.126.0/24`. El Ciber no promete un rango configurable ni prevención automática de colisiones. Si coincide con una red existente, detén la prueba y revisa el diseño de direccionamiento antes de continuar.

Esta alpha mantiene desactivados el descubrimiento P2P automático, STUN y hole punching también en VPN. Solo inicia el conector explícito al nodo, por lo que no promete conexiones directas automáticas entre amigos ni rendimiento equivalente a una malla final.

No compartimos automáticamente la subred doméstica, ni configuramos salida general a Internet, DNS del sistema, UPnP o captura de broadcasts de interfaces físicas. No se añade una regla de firewall por ti. Limita las excepciones a tu juego y a la interfaz virtual, según el sistema.

La red virtual no proporciona aislamiento por juego. Otros participantes pueden alcanzar servicios según las reglas de tu equipo. Invita solo a personas de confianza y cierra los servicios que no deban estar accesibles.

## Invitaciones y expulsión

Las invitaciones son portadoras de acceso compartido. No caducan ni se invalidan al borrar una sala local. No hay roles o expulsión individual en esta versión. Si se filtra una invitación, deja de usar esa red y crea una sala con nueva identidad; coordina el cambio con todos. El diseño futuro incorpora credenciales temporales del motor.

## Actualizar y volver atrás

- Desconecta y cierra El Ciber antes de actualizar. No sobrescribas un proceso en ejecución.
- Conserva el ejecutable anterior y una copia privada de los datos locales. **La copia contiene secretos.**
- El descargador del motor se niega a sobrescribir un directorio existente; prueba la nueva versión en otra carpeta y no cambies la versión fijada sin validar compatibilidad.
- Para volver atrás, usa el ejecutable y el motor anteriores con una copia compatible de los datos. No se promete migración automática entre esquemas futuros.
- Para retirar el cliente, desconecta y cierra el proceso; los archivos de la app pueden retirarse sin desinstalar servicios, porque El Ciber no instala ninguno. Eliminar los datos de usuario es una decisión separada y no revoca invitaciones remotas.

## Datos locales

Por defecto se usa la carpeta de configuración de usuario del sistema, en el subdirectorio `elciber`: `%AppData%\elciber` en Windows y `$XDG_CONFIG_HOME/elciber` o `~/.config/elciber` en Linux. `--data-dir` permite elegir una carpeta de pruebas. No uses carpetas compartidas, repositorios Git ni sincronización pública.

El cierre normal detiene el motor que inició el cliente. Cerrar solo la pestaña del navegador no equivale a cerrar el proceso. La recuperación después de una terminación forzada o un apagado sigue pendiente de validación.
