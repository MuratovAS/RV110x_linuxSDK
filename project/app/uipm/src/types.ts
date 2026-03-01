export type DeviceType = 'hub' | 'device';

export interface UsbipDevice {
  busid: string;
  vendorId: string;
  productId: string;
  name: string;
  port: number;
  occupied?: boolean;
}

export interface USBDevice {
  id: string;
  name: string;
  type: DeviceType;
  vendorId?: string;
  productId?: string;
  isBusy?: boolean;
  children?: USBDevice[];
}

export interface HubPort {
  id: number;
  power: boolean;
  devices: USBDevice[];
}

export type NetworkMode = 'dhcp' | 'static';

export interface EthernetConfig {
  blocked?: boolean;
  mode: NetworkMode;
  ip?: string;
  mask?: string;
  gateway?: string;
  dns?: string;
}

export type WiFiSecurity = 'wpa2' | 'open';

export interface WifiNetwork {
  ssid: string;
  signal: number; // dBm
  security: WiFiSecurity;
}

export interface WiFiConfig {
  blocked?: boolean;
  enabled: boolean;
  ssid?: string;
  password?: string;
  ip?: string;
  security?: WiFiSecurity;
}

export interface VPNConfig {
  blocked?: boolean;
  enabled: boolean;
  type: 'wireguard' | 'tailscale';
  status: 'connected' | 'disconnected' | 'connecting';
  config?: string; // For WireGuard
  // For Tailscale
  preauthkey?: string;
  exitNode?: boolean;
  serverUrl?: string;
}
