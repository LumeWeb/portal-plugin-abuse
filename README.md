# Portal Template Plugin

A template plugin for the Portal framework that demonstrates core functionality and best practices.

## Features

- Complete REST API with OpenAPI/Swagger documentation
- Frontend single-page application
- Database migrations for MySQL and SQLite
- Email notifications using templates
- File upload handling with progress tracking
- Workflow system integration
- Access control and authentication
- Configuration management

## Installation

1. Build using one of these methods:

a. Using xportal (recommended):
```bash
xportal build --with github.com/yourusername/portal-plugin-template@latest
```

b. Manual build (fallback):
```go
import (
    _ "github.com/yourusername/portal-plugin-template" // Import the plugin
)
```

The plugin will be automatically registered when Portal starts.

## Configuration

The plugin uses the following configuration structure in your Portal config file:

```yaml
template-plugin:
  storage_path: "data/template"  # Path to store protocol data
  max_items: 1000               # Maximum number of items to store
  cache_enabled: true           # Whether to enable caching
  api:
    items_per_page: 10         # Number of items per page in list responses
    search_limit: 100          # Maximum number of search results
```

## API Endpoints

The plugin provides the following REST API endpoints:

- `GET /api/items` - List all items (paginated)
- `POST /api/items` - Create a new item
- `GET /api/items/{id}` - Get a specific item
- `PUT /api/items/{id}` - Update an item
- `DELETE /api/items/{id}` - Delete an item
- `GET /api/items/search` - Search items
- `GET /api/items/protected` - List protected items (requires authentication)
- `GET /api/uploads/{id}` - Get upload status

Full API documentation is available at `template.{your-portal-domain}/swagger` when the plugin is running, where:
- `template` is the plugin's hardcoded subdomain
- `{your-portal-domain}` is your Portal instance domain

## Development

### Project Structure

```
.
├── build/              # Build information
├── internal/           # Internal package code
│   ├── api/           # REST API implementation
│   ├── config/        # Configuration structures
│   ├── db/            # Database models and migrations
│   ├── protocol/      # Protocol implementation
│   ├── service/       # Service implementations
│   ├── templates/     # Email templates
│   └── webapp/        # Frontend application
└── plugin.go          # Plugin entry point
```

### Adding New Features

1. Define models in `internal/db/models/`
2. Create migrations in `internal/db/migrations/`
3. Implement services in `internal/service/`
4. Add API endpoints in `internal/api/`
5. Update frontend in `internal/webapp/`

### Testing

Run the test suite:

```bash
go test ./...
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

For support, please open an issue in the GitHub repository or contact the maintainers.
