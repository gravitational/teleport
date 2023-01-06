// Each process creates its own key pair. The public key is saved to disk under the specified
// filename, the private key stays in the memory.
//
// `Renderer` and `Tshd` file names are also used on the tshd side.
export enum GrpcCertName {
  Renderer = 'renderer.crt',
  Tshd = 'tshd.crt',
  Shared = 'shared.crt',
}
