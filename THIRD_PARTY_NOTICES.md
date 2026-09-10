# Dependencias y procedencia

## Código propio

El cliente Go, la interfaz, los iconos vectoriales y la documentación de este repositorio se publican con licencia MIT. Los iconos y la identidad gráfica se han dibujado para este proyecto. Las capturas muestran su interfaz real con datos locales de prueba, no partidas reales.

## EasyTier · motor externo

- Proyecto: <https://github.com/EasyTier/EasyTier>.
- Versión de integración fijada: **v2.6.4**.
- Licencia declarada por la fuente: **LGPL-3.0**, véase [LICENSE del tag](https://github.com/EasyTier/EasyTier/blob/v2.6.4/LICENSE).
- Código fuente correspondiente: <https://github.com/EasyTier/EasyTier/tree/v2.6.4>.
- Artefactos oficiales: <https://github.com/EasyTier/EasyTier/releases/tag/v2.6.4>.

El Ciber no es un fork de EasyTier ni copia o enlaza su implementación. Lo ejecuta como proceso separado e intercambia configuración y estado mediante su CLI. Este repositorio no contiene sus binarios, código ni controladores. Los scripts opcionales descargan directamente los artefactos del proyecto original y verifican el SHA-256 fijado. El checksum comprueba integridad respecto al artefacto publicado, **no equivale a una auditoría ni a una firma independiente**.

El paquete oficial de Windows incluye Wintun y otros componentes de red con sus propias condiciones. Nuestro descargador solo selecciona `easytier-core.exe`, `easytier-cli.exe` y `wintun.dll`; no instala ni activa componentes de captura física. Consulta [Wintun](https://www.wintun.net/) y los avisos del proveedor antes de redistribuir. La licencia MIT de El Ciber no sustituye las licencias de terceros. Una futura distribución conjunta necesitará revisar e incluir las obligaciones de todos sus componentes.

## Instalador web

El candidato NSIS automatiza la descarga del motor desde sus publicaciones oficiales. No incluye binarios EasyTier/Wintun dentro del EXE. El cliente y el descargador propios incorporan los avisos de Go; el instalador incluye referencias de licencia y procedencia en `packaging/windows/THIRD-PARTY.txt`. No se ha publicado una release ni una distribución conjunta offline.

## Herramientas de desarrollo

- Go: licencia BSD-3-Clause; [licencia](https://go.dev/LICENSE).
- Playwright: Apache-2.0; [proyecto](https://github.com/microsoft/playwright). Solo para pruebas; no viaja dentro del cliente.
- Fuentes del sistema: no se distribuyen archivos de fuentes ni se consultan CDNs.

Hamachi es una referencia funcional, no una afiliación. Las marcas de juegos que puedan mencionarse pertenecen a sus respectivos propietarios; nombrarlas no acredita compatibilidad.
