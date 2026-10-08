# Cities — Social Simulator

Simulador social 8-bit donde se gestiona el crecimiento, economía y crisis de una ciudad con IA. Cada 30 segundos, una IA (Gemini 2.5) genera iniciativas políticas únicas basadas en el estado actual de la ciudad.

## Qué resuelve

Crear un juego simulador de ciudades donde una IA actúa como un consejo político dinámico, generando iniciativas de forma autónoma según el estado económico, desempleo, felicidad y recursos de la ciudad.

## Para quién

Jugadores interesados en simuladores de gestión urbana con economía compleja y toma de decisiones con IA.

## Características

- **Gobernanza con IA**: Consejo IA (Gemini 2.5) genera iniciativas únicas cada 30 segundos
- **Sistema económico**: Fábricas producen materiales, mercados generan alimentos, laboratorios desarrollan medicinas
- **Recursos cuantitativos**: Alimento, metal, materiales, medicinas
- **Ruinas y recuperación**: Las ciudades pueden colapsar por quiebra o emigración, pero pueden ser recuperadas
- **HUD en tiempo real**: Cliente Phaser.js responsive con propuestas interactivas

## Cómo instalar

Requisitos:
- Go 1.18+
- PostgreSQL 12+
- Redis 6+
- Docker Compose

```bash
git clone <repo>
cd Cities
docker-compose up -d
go mod download
```

## Cómo correr

```bash
# Backend Go
go run ./cmd/main.go
```

Acceso: `http://localhost:8080`

## Cómo probar

No hay tests automáticos documentados. Se prueba manualmente en el navegador.

## Cómo desplegar

Docker Compose está configurado para levantar la stack completa. Revisar `docker-compose.yml` para configuración de producción.

## Stack

Go, Phaser.js, PostgreSQL, Redis, Docker

## Estado

En reposo

Última actividad: 2026-03-19

## Relacionado con

OpenRouter, Gemini API

## Notas

Proyecto educativo/experimental de simulación urbana con IA. No tiene roadmap activo.
