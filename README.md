# fip-metadata 🎵📻

Welcome to the `fip-metadata` repository! This project, created by [@harperreed](https://github.com/harperreed), provides a simple API for fetching metadata from various FIP radio stations. 🌍🎧

## Repository Structure 📂

The repository is structured as follows:

```
fip-metadata/
├── Dockerfile
├── README.md
├── fly.toml
├── go.mod
├── go.sum
├── main.go
└── static
    └── index.html
```

## Getting Started 🚀

To get started with the `fip-metadata` API, follow these steps:

1. Clone the repository: `git clone https://github.com/harperreed/fip-metadata.git` 📥
2. Navigate to the project directory: `cd fip-metadata` 📂
3. Build the Docker image: `docker build -t fip-metadata .` 🛠️
4. Run the Docker container: `docker run -p 8080:8080 fip-metadata` 🏃‍♂️
5. Access the API at `http://localhost:8080/api/metadata/{param}` 🌐

Replace `{param}` with one of the available station identifiers listed in the API documentation. 📻

## API Documentation 📚

For detailed information on how to use the API and the available endpoints, please refer to the API documentation at `http://localhost:8080/` when running the API locally. 🔍

## Contributing 👥

Contributions to the `fip-metadata` project are always welcome! If you find a bug, have a feature request, or want to improve the code, please feel free to open an issue or submit a pull request. 🙌

## License 📜

This project is open-source and available under the [MIT License](https://opensource.org/licenses/MIT). Feel free to use, modify, and distribute the code as per the terms of the license. 📝

## Acknowledgements 🙏

Special thanks to the FIP radio network for providing the metadata API and to the open-source community for their valuable contributions and inspiration. 💕

---

Thank you for checking out the `fip-metadata` repository! If you have any questions or feedback, please don't hesitate to reach out. Happy coding! 😄🎉
