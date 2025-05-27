{
  description = "Immich Go Backend development environment with pinned gRPC tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        
        # Pin specific versions of protoc generators
        # You can override these with specific commits/versions
        protoc-gen-go-version = "v1.32.0";
        protoc-gen-go-grpc-version = "v1.3.0";
        
      in
      {
        devShells.default = pkgs.mkShell {
          name = "immich-go-backend";
          
          packages = with pkgs; [
            # Go toolchain
            go  # Use default Go version from Nix 25.05
            
            # Protocol Buffers
            protobuf
            protoc-gen-go
            protoc-gen-go-grpc
            grpc-gateway

            # SQL
            sqlc
            
            # Buf for modern protobuf workflow
            buf
            
            # gRPC development tools
            grpcurl
            grpc-tools
            
            # Additional development tools
            git
            curl
            jq
            postgresql  # For database development
          ];

          shellHook = ''
            echo "🚀 Immich Go Backend Development Environment (Nix Flake 25.05)"
            echo ""
            echo "📦 Pinned tool versions:"
            echo "   - Go:                 $(go version | cut -d' ' -f3)"
            echo "   - protoc:             $(protoc --version)"
            echo "   - buf:                $(buf --version 2>/dev/null || echo 'buf available')"
            echo "   - protoc-gen-go:      $(protoc-gen-go --version)"
            echo "   - protoc-gen-go-grpc: $(protoc-gen-go-grpc --version)"
            echo ""
            echo "🔧 Quick commands:"
            echo "   buf generate          # Generate Go code from .proto files"
            echo "   go mod tidy           # Clean up Go modules"
            echo "   go run main.go        # Run the server"
            echo ""
            
            # Protocol Buffers include path
            export PROTOC_INCLUDE="${pkgs.protobuf}/include"
          '';
          
          # Environment variables
          GO111MODULE = "on";
          CGO_ENABLED = "0";
        };
      });
}
