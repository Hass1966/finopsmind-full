#!/bin/bash
set -e

echo "🚀 Starting FinOpsMind..."

# Check for .env
if [ ! -f .env ]; then
    echo "📋 Creating .env from .env.example..."
    cp .env.example .env
    echo "⚠️  Please edit .env with your settings, then run this script again."
    exit 0
fi

# Start services
echo "🐳 Starting Docker services..."
docker compose up -d

echo ""
echo "⏳ Waiting for services to be ready..."
sleep 10

echo ""
echo "✅ FinOpsMind is running!"
echo ""
echo "📊 Dashboard:  http://localhost:3000"
echo "🔌 API:        http://localhost:8080/api/v1"
echo "🏥 Health:     http://localhost:8080/health"
echo "🤖 ML Sidecar: http://localhost:8000/health"
echo ""
echo "📝 View logs:  docker compose logs -f"
echo "🛑 Stop:       docker compose down"
