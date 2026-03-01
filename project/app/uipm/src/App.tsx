import React, { useState, useEffect, useRef } from 'react';
import { 
  Usb, 
  Power, 
  Network, 
  ShieldCheck, 
  Wifi, 
  Globe, 
  Cpu, 
  Settings2, 
  Activity,
  Lock,
  Server,
  Edit2,
  Info
} from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { AreaChart, Area, ResponsiveContainer } from 'recharts';
import {
  USBDevice,
  HubPort,
  EthernetConfig,
  WiFiConfig,
  WiFiSecurity,
  WifiNetwork,
  VPNConfig,
  UsbipDevice
} from './types';

const INITIAL_PORTS: HubPort[] = [
  { id: 1, power: true, devices: [] },
  { id: 2, power: true, devices: [] },
  { id: 3, power: false, devices: [] },
  { id: 4, power: true, devices: [] },
];

const DeviceTree: React.FC<{ device: USBDevice; depth?: number }> = ({ device, depth = 0 }) => {
  return (
    <div className="flex flex-col">
      <div 
        className={`flex items-center gap-2 py-1.5 px-2 rounded-md hover:bg-slate-50 transition-colors ${depth > 0 ? 'ml-4 border-l border-slate-200' : ''}`}
      >
        {device.type === 'hub' ? (
          <Server className={`w-4 h-4 ${device.isBusy ? 'text-maroon' : 'text-slate-400'}`} />
        ) : (
          <Usb className={`w-4 h-4 ${device.isBusy ? 'text-maroon' : 'text-slate-400'}`} />
        )}
        <div className="flex flex-col">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-slate-700">{device.name}</span>
          </div>
          {device.vendorId && (
            <span className="text-[10px] text-slate-400 font-mono">
              {device.id} [{device.vendorId}:{device.productId}]
            </span>
          )}
        </div>
      </div>
      {device.children?.map((child) => (
        <DeviceTree key={child.id} device={child} depth={depth + 1} />
      ))}
    </div>
  );
};

const CombinedNetworkChart = ({ history }: { history: {rx: number, tx: number}[] }) => {
  return (
    <div className="w-full h-full" style={{ 
      maskImage: 'linear-gradient(to right, transparent 0%, transparent 50%, rgba(0, 0, 0, 0.5) 100%)', 
      WebkitMaskImage: 'linear-gradient(to right, transparent 0%, transparent 50%, rgba(0, 0, 0, 0.5) 100%)' 
    }}>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={history} margin={{ top: 0, right: 0, left: 0, bottom: 0 }}>
          <Area 
            type="monotone" 
            dataKey="rx" 
            stroke="#94a3b8" 
            fill="#94a3b8" 
            fillOpacity={0.1} 
            strokeWidth={1}
            isAnimationActive={false}
          />
          <Area 
            type="monotone" 
            dataKey="tx" 
            stroke="#ea580c"
            fill="#ea580c"
            fillOpacity={0.15} 
            strokeWidth={1}
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
};

const SignalBars: React.FC<{ signal: number }> = ({ signal }) => {
  const strength = signal >= -55 ? 4 : signal >= -65 ? 3 : signal >= -75 ? 2 : 1;
  return (
    <div className="flex items-end gap-px h-3">
      {[1, 2, 3, 4].map(i => (
        <div
          key={i}
          className={`w-1 rounded-sm ${i <= strength ? 'bg-maroon' : 'bg-slate-200'}`}
          style={{ height: `${i * 25}%` }}
        />
      ))}
    </div>
  );
};

const IpDisplay: React.FC<{
  info: { ipv4: string[]; ipv6: string[] } | null;
  fallback: string;
}> = ({ info, fallback }) => {
  const all = info?.ipv4 ?? [];
  if (all.length === 0) return <>{fallback}</>;
  return <>{all.join(' · ')}</>;
};

type IfaceMap = Record<string, { ipv4: string[]; ipv6: string[]; mac: string }>;

const InterfaceDetails: React.FC<{ patterns: RegExp[]; ifaces: IfaceMap }> = ({ patterns, ifaces }) => {
  const entry = Object.entries(ifaces).find(([name]) => patterns.some(p => p.test(name)));
  if (!entry) return <p className="text-[11px] text-slate-400 py-1">No active interface</p>;
  const [name, info] = entry;
  const rows: { label: string; value: string }[] = [
    { label: 'iface', value: name },
    ...(info.mac ? [{ label: 'MAC', value: info.mac }] : []),
    ...info.ipv4.map((ip, i) => ({ label: i === 0 ? 'IPv4' : '', value: ip })),
    ...info.ipv6.map((ip, i) => ({ label: i === 0 ? 'IPv6' : '', value: ip })),
  ];
  return (
    <div className="space-y-1 text-[11px] font-mono">
      {rows.map((r, i) => (
        <div key={i} className="flex gap-3">
          <span className="text-slate-400 w-10 shrink-0">{r.label}</span>
          <span className="text-slate-600 break-all">{r.value}</span>
        </div>
      ))}
    </div>
  );
};

export default function App() {
  const [networkHistory, setNetworkHistory] = useState<{rx: number, tx: number}[]>(
    Array.from({ length: 60 }, () => ({ rx: 0, tx: 0 }))
  );

  useEffect(() => {
    const fetchNetwork = async () => {
      try {
        const res = await fetch('/api/network');
        const data = await res.json();
        setNetworkHistory(prev => [...prev.slice(1), { rx: data.rx, tx: data.tx }]);
      } catch {
        // keep previous values on error
      }
    };
    fetchNetwork();
    const interval = setInterval(fetchNetwork, 1000);
    return () => clearInterval(interval);
  }, []);

  const [ports, setPorts] = useState<HubPort[]>(INITIAL_PORTS);
  const [usbDevices, setUsbDevices] = useState<UsbipDevice[]>([]);

  useEffect(() => {
    const fetchUsb = () => {
      fetch('/api/usb/devices')
        .then(r => r.json())
        .then((data: UsbipDevice[]) => setUsbDevices(data ?? []))
        .catch(() => {});
    };
    fetchUsb();
    const interval = setInterval(fetchUsb, 5000);
    return () => clearInterval(interval);
  }, []);

  const [savedEthConfig, setSavedEthConfig] = useState<EthernetConfig>({
    mode: 'dhcp',
    ip: '192.168.1.142',
    gateway: '192.168.1.1'
  });
  const [ethConfig, setEthConfig] = useState<EthernetConfig>({ ...savedEthConfig });
  const [isEthEditing, setIsEthEditing] = useState(false);

  const [savedWifiConfig, setSavedWifiConfig] = useState<WiFiConfig>({
    enabled: false,
    ssid: 'Home_Network_5G',
    password: '',
    ip: '192.168.1.143'
  });
  const [wifiConfig, setWifiConfig] = useState<WiFiConfig>({ ...savedWifiConfig });
  const [isWifiEditing, setIsWifiEditing] = useState(false);
  const [wifiNetworks, setWifiNetworks] = useState<WifiNetwork[]>([]);
  const [wifiScanning, setWifiScanning] = useState(false);

  const scanWifi = () => {
    setWifiScanning(true);
    fetch('/api/wifi/scan')
      .then(r => r.json())
      .then((data: WifiNetwork[]) => setWifiNetworks(data))
      .catch(() => setWifiNetworks([]))
      .finally(() => setWifiScanning(false));
  };

  React.useEffect(() => {
    if (isWifiEditing) {
      scanWifi();
    } else {
      setWifiNetworks([]);
    }
  }, [isWifiEditing]);
  
  const [savedWgConfig, setSavedWgConfig] = useState<VPNConfig>({
    enabled: false,
    type: 'wireguard',
    status: 'disconnected',
    config: '[Interface]\nPrivateKey = ...\nAddress = 10.0.0.5/32'
  });
  const [wgConfig, setWgConfig] = useState<VPNConfig>({ ...savedWgConfig });
  const [isWgEditing, setIsWgEditing] = useState(false);

  const [savedTsConfig, setSavedTsConfig] = useState<VPNConfig>({
    enabled: false,
    type: 'tailscale',
    status: 'disconnected',
    preauthkey: '',
    exitNode: false,
    serverUrl: 'https://controlplane.tailscale.com'
  });
  const [tsConfig, setTsConfig] = useState<VPNConfig>({ ...savedTsConfig });
  const [isTsEditing, setIsTsEditing] = useState(false);

  const saveToServer = (overrides: {
    ports?: HubPort[];
    ethernet?: EthernetConfig;
    wifi?: WiFiConfig;
    wireguard?: VPNConfig;
    tailscale?: VPNConfig;
  } = {}) => {
    const eth = overrides.ethernet ?? savedEthConfig;
    const wifi = overrides.wifi ?? savedWifiConfig;
    const wg = overrides.wireguard ?? savedWgConfig;
    const ts = overrides.tailscale ?? savedTsConfig;
    const portsList = overrides.ports ?? ports;
    fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ports: portsList.map(p => ({ id: p.id, power: p.power })),
        ethernet: eth,
        wifi: wifi,
        wireguard: { blocked: wg.blocked, enabled: wg.enabled, config: wg.config },
        tailscale: { blocked: ts.blocked, enabled: ts.enabled, preauthkey: ts.preauthkey, exitNode: ts.exitNode, serverUrl: ts.serverUrl },
      }),
    }).catch(console.error);
  };

  const [appVersion, setAppVersion] = React.useState('...');

  React.useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then((data: any) => {
        if (data.ethernet) {
          setSavedEthConfig(data.ethernet);
          setEthConfig(data.ethernet);
        }
        if (data.wifi) {
          setSavedWifiConfig(data.wifi);
          setWifiConfig(data.wifi);
        }
        if (data.wireguard) {
          const wg: VPNConfig = { type: 'wireguard', status: 'disconnected', ...data.wireguard };
          setSavedWgConfig(wg);
          setWgConfig(wg);
        }
        if (data.tailscale) {
          const ts: VPNConfig = { type: 'tailscale', status: 'disconnected', ...data.tailscale };
          setSavedTsConfig(ts);
          setTsConfig(ts);
        }
        if (data.ports) {
          setPorts(prev => prev.map(p => {
            const saved = data.ports.find((sp: { id: number }) => sp.id === p.id);
            return saved ? { ...p, power: saved.power } : p;
          }));
        }
      })
      .catch(console.error);
  }, []);

  React.useEffect(() => {
    fetch('/api/version')
      .then(r => r.json())
      .then(d => setAppVersion(d.version))
      .catch(() => setAppVersion('unknown'));
  }, []);

  const [ifaces, setIfaces] = React.useState<IfaceMap>({});
  const [isEthInfo, setIsEthInfo] = useState(false);
  const [isWifiInfo, setIsWifiInfo] = useState(false);
  const [isTsInfo, setIsTsInfo] = useState(false);
  const [isWgInfo, setIsWgInfo] = useState(false);

  React.useEffect(() => {
    const fetch_ = () =>
      fetch('/api/interfaces')
        .then(r => r.json())
        .then(setIfaces)
        .catch(() => {});
    fetch_();
    const t = setInterval(fetch_, 10000);
    return () => clearInterval(t);
  }, []);

  const ifaceIPs = (patterns: RegExp[]): { ipv4: string[]; ipv6: string[] } | null => {
    for (const [name, info] of Object.entries(ifaces)) {
      if (patterns.some(p => p.test(name))) return info;
    }
    return null;
  };

  const hasIP = (info: { ipv4: string[]; ipv6: string[] } | null) =>
    (info?.ipv4?.length ?? 0) > 0;

  const [cmdErrors, setCmdErrors] = useState<{ id: number; message: string }[]>([]);
  const cmdErrorIdRef = useRef(0);

  useEffect(() => {
    const poll = () => {
      fetch('/api/errors')
        .then(r => r.json())
        .then((data: { message: string }[]) => {
          if (!data || data.length === 0) return;
          data.forEach(({ message }) => {
            const id = ++cmdErrorIdRef.current;
            setCmdErrors(prev => [...prev, { id, message }]);
            setTimeout(() => {
              setCmdErrors(prev => prev.filter(e => e.id !== id));
            }, 15000);
          });
        })
        .catch(() => {});
    };
    const interval = setInterval(poll, 5000);
    return () => clearInterval(interval);
  }, []);

  const [metrics, setMetrics] = React.useState({ cpu: 0, ram: 0, uptime: '...' });

  React.useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const res = await fetch('/api/metrics');
        const data = await res.json();
        setMetrics(data);
      } catch {
        // keep previous values on error
      }
    };
    fetchMetrics();
    const interval = setInterval(fetchMetrics, 3000);
    return () => clearInterval(interval);
  }, []);

  const isEthChanged = JSON.stringify(ethConfig) !== JSON.stringify(savedEthConfig);
  const isWifiChanged = JSON.stringify(wifiConfig) !== JSON.stringify(savedWifiConfig);
  const isWgChanged = JSON.stringify(wgConfig) !== JSON.stringify(savedWgConfig);
  const isTsChanged = JSON.stringify(tsConfig) !== JSON.stringify(savedTsConfig);

  const handleApplyEth = () => {
    const next = { ...ethConfig };
    setSavedEthConfig(next);
    setIsEthEditing(false);
    saveToServer({ ethernet: next });
  };

  const handleApplyWifi = () => {
    const next = { ...wifiConfig };
    setSavedWifiConfig(next);
    setIsWifiEditing(false);
    saveToServer({ wifi: next });
  };

  const handleApplyWg = () => {
    const next = { ...wgConfig };
    setSavedWgConfig(next);
    setIsWgEditing(false);
    saveToServer({ wireguard: next });
  };

  const handleApplyTs = () => {
    const next = { ...tsConfig };
    setSavedTsConfig(next);
    setIsTsEditing(false);
    saveToServer({ tailscale: next });
  };

  const togglePortPower = (id: number) => {
    setPorts(prev => {
      const next = prev.map(p => p.id === id ? { ...p, power: !p.power } : p);
      saveToServer({ ports: next });
      return next;
    });
  };

  return (
    <div className="min-h-screen flex flex-col max-w-6xl mx-auto p-4 md:p-8 gap-8">
      {/* Network Status Header */}
      <header className="relative flex flex-wrap items-center justify-between gap-4 bg-white p-4 rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
        {/* Background Chart */}
        <div className="absolute inset-0 z-0 pointer-events-none">
          <CombinedNetworkChart history={networkHistory} />
        </div>

        <div className="relative z-10 flex items-center gap-3">
          <div className="p-2 bg-maroon/10 rounded-xl">
            <Activity className="w-6 h-6 text-maroon" />
          </div>
          <div>
            <h1 className="text-xl font-bold tracking-tight text-slate-900">USBIP Hub Manager</h1>
          </div>
        </div>

        <div className="relative z-10 flex items-center gap-6 pr-2">
          <div className="flex items-center gap-4 text-[10px] font-bold uppercase tracking-tight">
            <div className="flex flex-col items-end">
              <span className="text-slate-400 text-[8px]">Receive</span>
              <span className="text-slate-600 font-mono">{networkHistory[networkHistory.length - 1].rx.toFixed(1)} MB</span>
            </div>
            <div className="flex flex-col items-end">
              <span className="text-maroon text-[8px]">Transmit</span>
              <span className="text-slate-600 font-mono">{networkHistory[networkHistory.length - 1].tx.toFixed(1)} MB</span>
            </div>
          </div>
        </div>
      </header>

      <AnimatePresence>
        {cmdErrors.map(err => (
          <motion.div
            key={err.id}
            initial={{ opacity: 0, y: -8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            className="flex items-center justify-between gap-3 bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-xl"
          >
            <span className="font-mono text-xs break-all">{err.message}</span>
            <button
              onClick={() => setCmdErrors(prev => prev.filter(e => e.id !== err.id))}
              className="shrink-0 text-red-400 hover:text-red-600 text-lg leading-none"
            >
              ×
            </button>
          </motion.div>
        ))}
      </AnimatePresence>

      <main className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        
        {/* Block 1: USB Ports & Devices */}
        <section className="lg:col-span-8 flex flex-col gap-6">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold flex items-center gap-2">
              <Usb className="w-5 h-5 text-maroon" />
              USB Ports & Devices
            </h2>
            <span className="text-xs font-medium text-slate-400 uppercase tracking-widest">4-Port Controller</span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {ports.map((port) => (
              <div key={port.id} className="card p-5 flex flex-col gap-4 min-h-[200px]">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`w-8 h-8 rounded-lg flex items-center justify-center font-bold text-sm ${port.power ? 'bg-maroon text-white' : 'bg-slate-100 text-slate-400'}`}>
                      {port.id}
                    </div>
                    <span className="font-bold text-slate-700">Port {port.id}</span>
                  </div>
                  
                  <button 
                    onClick={() => togglePortPower(port.id)}
                    className={`toggle-switch ${port.power ? 'bg-maroon' : 'bg-slate-200'}`}
                  >
                    <span className={`toggle-thumb ${port.power ? 'translate-x-6' : 'translate-x-1'}`} />
                  </button>
                </div>

                <div className="flex-1 bg-slate-50/50 rounded-lg p-3 border border-dashed border-slate-200">
                  {!port.power ? (
                    <div className="h-full flex flex-col items-center justify-center text-slate-400 gap-2 opacity-50">
                      <Power className="w-6 h-6" />
                      <span className="text-xs font-medium">Power Disabled</span>
                    </div>
                  ) : usbDevices.filter(d => d.port === port.id).length === 0 ? (
                    <div className="h-full flex flex-col items-center justify-center text-slate-400 gap-2">
                      <div className="w-1.5 h-1.5 rounded-full bg-slate-300 animate-pulse" />
                      <span className="text-xs font-medium">Empty</span>
                    </div>
                  ) : (
                    <div className="flex flex-col gap-1">
                      {usbDevices.filter(d => d.port === port.id).map(d => (
                        <DeviceTree key={d.busid} device={{
                          id: d.busid,
                          name: d.name || d.busid,
                          type: 'device',
                          vendorId: d.vendorId,
                          productId: d.productId,
                          isBusy: d.occupied,
                        }} />
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Sidebar Blocks */}
        <aside className="lg:col-span-4 flex flex-col gap-8">
          
          {/* Block 2: Network Settings */}
          <section className="flex flex-col gap-4">
            <h2 className="text-lg font-bold flex items-center gap-2">
              <Globe className="w-5 h-5 text-maroon" />
              Network
            </h2>
            
            <div className="flex flex-col gap-4">
              {/* Ethernet */}
              {!savedEthConfig.blocked && <div className="card p-5 flex flex-col gap-4 group">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center transition-colors ${hasIP(ifaceIPs([/^eth\d/, /^en[opsx]?\d/, /^en\d/])) ? 'bg-maroon/10' : 'bg-slate-100'}`}>
                      <Network className={`w-5 h-5 transition-colors ${hasIP(ifaceIPs([/^eth\d/, /^en[opsx]?\d/, /^en\d/])) ? 'text-maroon' : 'text-slate-500'}`} />
                    </div>
                    <div className="leading-tight">
                      <div className="flex items-center gap-2">
                        <h3 className="font-bold text-slate-800">Ethernet</h3>
                        <button
                          onClick={() => setIsEthEditing(!isEthEditing)}
                          className={`p-1 rounded-md transition-all ${isEthEditing ? 'bg-maroon/10 text-maroon' : 'hover:bg-slate-100 text-slate-400'} ${isEthEditing || isEthInfo ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}
                        >
                          <Edit2 className="w-3 h-3" />
                        </button>
                        <button
                          onClick={() => setIsEthInfo(!isEthInfo)}
                          className={`p-1 rounded-md transition-all ${isEthInfo ? 'bg-maroon/10 text-maroon' : 'hover:bg-slate-100 text-slate-400'} ${isEthEditing || isEthInfo ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}
                        >
                          <Info className="w-3 h-3" />
                        </button>
                      </div>
                      <span className="text-xs font-mono font-medium text-slate-500">
                        <IpDisplay
                          info={ifaceIPs([/^eth\d/, /^en[opsx]?\d/, /^en\d/])}
                          fallback="Disconnected"
                        />
                      </span>
                    </div>
                  </div>
                </div>

                <AnimatePresence>
                  {isEthEditing && (
                    <motion.div 
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="space-y-4 overflow-hidden"
                    >
                      <div className="flex p-1 bg-slate-100 rounded-lg">
                        <button 
                          onClick={() => setEthConfig({...ethConfig, mode: 'dhcp'})}
                          className={`flex-1 py-1.5 text-xs font-bold rounded-md transition-all ${ethConfig.mode === 'dhcp' ? 'bg-white shadow-sm text-maroon' : 'text-slate-500'}`}
                        >
                          DHCP
                        </button>
                        <button 
                          onClick={() => setEthConfig({...ethConfig, mode: 'static'})}
                          className={`flex-1 py-1.5 text-xs font-bold rounded-md transition-all ${ethConfig.mode === 'static' ? 'bg-white shadow-sm text-maroon' : 'text-slate-500'}`}
                        >
                          STATIC
                        </button>
                      </div>

                      {ethConfig.mode === 'static' && (
                        <div className="flex flex-col gap-4">
                          <div className="space-y-1">
                            <label className="text-[10px] font-bold text-slate-400 uppercase">IP Address</label>
                            <input 
                              type="text" 
                              className="input-field" 
                              value={ethConfig.ip || ''} 
                              onChange={(e) => setEthConfig({...ethConfig, ip: e.target.value})}
                              placeholder="192.168.1.100" 
                            />
                          </div>
                          <div className="space-y-1">
                            <label className="text-[10px] font-bold text-slate-400 uppercase">Gateway</label>
                            <input 
                              type="text" 
                              className="input-field" 
                              value={ethConfig.gateway || ''} 
                              onChange={(e) => setEthConfig({...ethConfig, gateway: e.target.value})}
                              placeholder="192.168.1.1" 
                            />
                          </div>
                        </div>
                      )}

                      {isEthChanged && (
                        <motion.button 
                          initial={{ opacity: 0, scale: 0.95 }}
                          animate={{ opacity: 1, scale: 1 }}
                          onClick={handleApplyEth}
                          className="btn-primary w-full text-xs py-1.5 shadow-md shadow-maroon/10"
                        >
                          Apply Changes
                        </motion.button>
                      )}
                    </motion.div>
                  )}
                </AnimatePresence>
                <AnimatePresence>
                  {isEthInfo && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="overflow-hidden"
                    >
                      <div className="pt-3 border-t border-slate-100">
                        <InterfaceDetails patterns={[/^eth\d/, /^en[opsx]?\d/, /^en\d/]} ifaces={ifaces} />
                      </div>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>}

              {/* WiFi */}
              {!savedWifiConfig.blocked && <div className="card p-5 flex flex-col gap-4 group">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center transition-colors ${hasIP(ifaceIPs([/^wlan\d/, /^wlp\d/, /^wl\d/])) ? 'bg-maroon/10' : 'bg-slate-100'}`}>
                      <Wifi className={`w-5 h-5 transition-colors ${hasIP(ifaceIPs([/^wlan\d/, /^wlp\d/, /^wl\d/])) ? 'text-maroon' : 'text-slate-500'}`} />
                    </div>
                    <div className="leading-tight">
                      <div className="flex items-center gap-2">
                        <h3 className="font-bold text-slate-800">WiFi</h3>
                        <button
                          onClick={() => setIsWifiEditing(!isWifiEditing)}
                          className={`p-1 rounded-md transition-all ${isWifiEditing ? 'bg-maroon/10 text-maroon' : 'hover:bg-slate-100 text-slate-400'} ${isWifiEditing || isWifiInfo ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}
                        >
                          <Edit2 className="w-3 h-3" />
                        </button>
                        <button
                          onClick={() => setIsWifiInfo(!isWifiInfo)}
                          className={`p-1 rounded-md transition-all ${isWifiInfo ? 'bg-maroon/10 text-maroon' : 'hover:bg-slate-100 text-slate-400'} ${isWifiEditing || isWifiInfo ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}
                        >
                          <Info className="w-3 h-3" />
                        </button>
                      </div>
                      <span className="text-xs font-mono font-medium text-slate-500">
                        {wifiConfig.enabled
                          ? <IpDisplay
                              info={ifaceIPs([/^wlan\d/, /^wlp\d/, /^wl\d/])}
                              fallback="Disconnected"
                            />
                          : 'Disconnected'}
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={() => {
                      const next = {...wifiConfig, enabled: !wifiConfig.enabled};
                      setWifiConfig(next);
                      setSavedWifiConfig(next);
                      saveToServer({ wifi: next });
                    }}
                    className={`toggle-switch ${wifiConfig.enabled ? 'bg-maroon' : 'bg-slate-200'}`}
                  >
                    <span className={`toggle-thumb ${wifiConfig.enabled ? 'translate-x-6' : 'translate-x-1'}`} />
                  </button>
                </div>

                <AnimatePresence>
                  {isWifiEditing && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="space-y-4 overflow-hidden"
                    >
                      {/* Network list */}
                      <div className="space-y-1">
                        <div className="flex items-center justify-between">
                          <label className="text-[10px] font-bold text-slate-400 uppercase">Available Networks</label>
                          <button
                            onClick={scanWifi}
                            disabled={wifiScanning}
                            className="text-[10px] font-bold text-maroon disabled:text-slate-400 hover:underline"
                          >
                            {wifiScanning ? 'Scanning…' : 'Scan'}
                          </button>
                        </div>
                        <div className="max-h-40 overflow-y-auto flex flex-col gap-0.5 rounded-lg border border-slate-200 p-1 bg-slate-50/50">
                          {wifiNetworks.length === 0 ? (
                            <p className="text-[10px] text-slate-400 text-center py-3">
                              {wifiScanning ? 'Scanning for networks…' : 'No networks found'}
                            </p>
                          ) : wifiNetworks.map(net => (
                            <button
                              key={net.ssid}
                              onClick={() => setWifiConfig({
                                ...wifiConfig,
                                ssid: net.ssid,
                                security: net.security as WiFiSecurity,
                              })}
                              className={`flex items-center justify-between px-2 py-1.5 rounded-md text-left transition-colors hover:bg-white ${wifiConfig.ssid === net.ssid ? 'bg-white border border-maroon/20 shadow-sm' : ''}`}
                            >
                              <span className="text-xs font-medium text-slate-700 truncate mr-2">{net.ssid}</span>
                              <div className="flex items-center gap-1.5 shrink-0">
                                <SignalBars signal={net.signal} />
                                {net.security !== 'open' && <Lock className="w-2.5 h-2.5 text-slate-400" />}
                              </div>
                            </button>
                          ))}
                        </div>
                      </div>

                      {/* SSID manual input */}
                      <div className="space-y-1">
                        <label className="text-[10px] font-bold text-slate-400 uppercase">SSID</label>
                        <input
                          type="text"
                          className="input-field"
                          value={wifiConfig.ssid || ''}
                          onChange={(e) => setWifiConfig({...wifiConfig, ssid: e.target.value})}
                          placeholder="Network name"
                        />
                      </div>

                      {/* Security type */}
                      <div className="space-y-1">
                        <label className="text-[10px] font-bold text-slate-400 uppercase">Security</label>
                        <div className="flex p-1 bg-slate-100 rounded-lg">
                          {(['wpa2', 'open'] as const).map(sec => (
                            <button
                              key={sec}
                              onClick={() => setWifiConfig({...wifiConfig, security: sec})}
                              className={`flex-1 py-1.5 text-[10px] font-bold rounded-md transition-all ${(wifiConfig.security ?? 'wpa2') === sec ? 'bg-white shadow-sm text-maroon' : 'text-slate-500'}`}
                            >
                              {sec === 'wpa2' ? 'WPA/WPA2-PSK' : 'Open'}
                            </button>
                          ))}
                        </div>
                      </div>

                      {/* Password — hidden for open networks */}
                      {(wifiConfig.security ?? 'wpa2') !== 'open' && (
                        <div className="space-y-1">
                          <label className="text-[10px] font-bold text-slate-400 uppercase">Password</label>
                          <input
                            type="password"
                            className="input-field"
                            value={wifiConfig.password || ''}
                            onChange={(e) => setWifiConfig({...wifiConfig, password: e.target.value})}
                            placeholder="••••••••"
                          />
                        </div>
                      )}

                      {isWifiChanged && (
                        <motion.button
                          initial={{ opacity: 0, scale: 0.95 }}
                          animate={{ opacity: 1, scale: 1 }}
                          onClick={handleApplyWifi}
                          className="btn-primary w-full text-xs py-1.5 shadow-md shadow-maroon/10"
                        >
                          Apply Changes
                        </motion.button>
                      )}
                    </motion.div>
                  )}
                </AnimatePresence>
                <AnimatePresence>
                  {isWifiInfo && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="overflow-hidden"
                    >
                      <div className="pt-3 border-t border-slate-100">
                        <InterfaceDetails patterns={[/^wlan\d/, /^wlp\d/, /^wl\d/]} ifaces={ifaces} />
                      </div>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>}
            </div>
          </section>

          {/* Block 3: VPN Settings */}
          <section className="flex flex-col gap-4">
            <h2 className="text-lg font-bold flex items-center gap-2">
              <ShieldCheck className="w-5 h-5 text-maroon" />
              VPN Services
            </h2>
            
            <div className="flex flex-col gap-4">
              {/* Tailscale */}
              {!savedTsConfig.blocked && <div className="card p-5 flex flex-col gap-4 group">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center transition-colors ${hasIP(ifaceIPs([/^tailscale/])) ? 'bg-maroon/10' : 'bg-slate-100'}`}>
                      <Settings2 className={`w-5 h-5 transition-colors ${hasIP(ifaceIPs([/^tailscale/])) ? 'text-maroon' : 'text-slate-500'}`} />
                    </div>
                    <div className="leading-tight">
                      <div className="flex items-center gap-2">
                        <h3 className="font-bold text-slate-800">Tailscale</h3>
                        <button
                          onClick={() => setIsTsEditing(!isTsEditing)}
                          className={`p-1 rounded-md transition-all ${isTsEditing ? 'bg-maroon/10 text-maroon' : 'hover:bg-slate-100 text-slate-400'} ${isTsEditing || isTsInfo ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}
                        >
                          <Edit2 className="w-3 h-3" />
                        </button>
                        <button
                          onClick={() => setIsTsInfo(!isTsInfo)}
                          className={`p-1 rounded-md transition-all ${isTsInfo ? 'bg-maroon/10 text-maroon' : 'hover:bg-slate-100 text-slate-400'} ${isTsEditing || isTsInfo ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}
                        >
                          <Info className="w-3 h-3" />
                        </button>
                      </div>
                      <span className="text-xs font-mono font-medium text-slate-500">
                        {tsConfig.enabled
                          ? <IpDisplay info={ifaceIPs([/^tailscale/])} fallback="Connecting…" />
                          : 'Disconnected'}
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={() => {
                      const next = {...tsConfig, enabled: !tsConfig.enabled, status: (!tsConfig.enabled ? 'connecting' : 'disconnected') as VPNConfig['status']};
                      setTsConfig(next);
                      setSavedTsConfig(next);
                      saveToServer({ tailscale: next });
                    }}
                    className={`toggle-switch ${tsConfig.enabled ? 'bg-maroon' : 'bg-slate-200'}`}
                  >
                    <span className={`toggle-thumb ${tsConfig.enabled ? 'translate-x-6' : 'translate-x-1'}`} />
                  </button>
                </div>

                <AnimatePresence>
                  {isTsEditing && (
                    <motion.div 
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="space-y-4 overflow-hidden"
                    >
                      <div className="space-y-1">
                        <label className="text-[10px] font-bold text-slate-400 uppercase">Pre-Auth Key</label>
                        <input 
                          type="password" 
                          className="input-field" 
                          placeholder="tskey-auth-..." 
                          value={tsConfig.preauthkey || ''}
                          onChange={(e) => setTsConfig({...tsConfig, preauthkey: e.target.value})}
                        />
                      </div>
                      
                      <div className="space-y-1">
                        <label className="text-[10px] font-bold text-slate-400 uppercase">Server URL</label>
                        <input 
                          type="text" 
                          className="input-field" 
                          placeholder="https://controlplane.tailscale.com" 
                          value={tsConfig.serverUrl || ''}
                          onChange={(e) => setTsConfig({...tsConfig, serverUrl: e.target.value})}
                        />
                      </div>

                      <div className="flex items-center justify-between p-2 bg-slate-50 rounded-lg border border-slate-100">
                        <span className="text-xs font-bold text-slate-600">Exit Node</span>
                        <button 
                          onClick={() => setTsConfig({...tsConfig, exitNode: !tsConfig.exitNode})}
                          className={`toggle-switch scale-75 ${tsConfig.exitNode ? 'bg-maroon' : 'bg-slate-200'}`}
                        >
                          <span className={`toggle-thumb ${tsConfig.exitNode ? 'translate-x-6' : 'translate-x-1'}`} />
                        </button>
                      </div>

                      {isTsChanged && (
                        <motion.button 
                          initial={{ opacity: 0, scale: 0.95 }}
                          animate={{ opacity: 1, scale: 1 }}
                          onClick={handleApplyTs}
                          className="btn-primary w-full text-xs py-1.5 shadow-md shadow-maroon/10"
                        >
                          Apply Changes
                        </motion.button>
                      )}
                    </motion.div>
                  )}
                </AnimatePresence>
                <AnimatePresence>
                  {isTsInfo && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="overflow-hidden"
                    >
                      <div className="pt-3 border-t border-slate-100">
                        <InterfaceDetails patterns={[/^tailscale/]} ifaces={ifaces} />
                      </div>
                    </motion.div>
                  )}
                </AnimatePresence>

              </div>}

              {/* WireGuard */}
              {!savedWgConfig.blocked && <div className="card p-5 flex flex-col gap-4 group">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center transition-colors ${hasIP(ifaceIPs([/^wg\d/])) ? 'bg-maroon/10' : 'bg-slate-100'}`}>
                      <Lock className={`w-5 h-5 transition-colors ${hasIP(ifaceIPs([/^wg\d/])) ? 'text-maroon' : 'text-slate-500'}`} />
                    </div>
                    <div className="leading-tight">
                      <div className="flex items-center gap-2">
                        <h3 className="font-bold text-slate-800">WireGuard</h3>
                        <button
                          onClick={() => setIsWgEditing(!isWgEditing)}
                          className={`p-1 rounded-md transition-all ${isWgEditing ? 'bg-maroon/10 text-maroon' : 'hover:bg-slate-100 text-slate-400'} ${isWgEditing || isWgInfo ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}
                        >
                          <Edit2 className="w-3 h-3" />
                        </button>
                        <button
                          onClick={() => setIsWgInfo(!isWgInfo)}
                          className={`p-1 rounded-md transition-all ${isWgInfo ? 'bg-maroon/10 text-maroon' : 'hover:bg-slate-100 text-slate-400'} ${isWgEditing || isWgInfo ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}`}
                        >
                          <Info className="w-3 h-3" />
                        </button>
                      </div>
                      <span className="text-xs font-mono font-medium text-slate-500">
                        {wgConfig.enabled
                          ? <IpDisplay
                              info={ifaceIPs([/^wg\d/])}
                              fallback="Connecting…"
                            />
                          : 'Disconnected'}
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={() => {
                      const next = {...wgConfig, enabled: !wgConfig.enabled, status: (!wgConfig.enabled ? 'connected' : 'disconnected') as VPNConfig['status']};
                      setWgConfig(next);
                      setSavedWgConfig(next);
                      saveToServer({ wireguard: next });
                    }}
                    className={`toggle-switch ${wgConfig.enabled ? 'bg-maroon' : 'bg-slate-200'}`}
                  >
                    <span className={`toggle-thumb ${wgConfig.enabled ? 'translate-x-6' : 'translate-x-1'}`} />
                  </button>
                </div>
                
                <AnimatePresence>
                  {isWgEditing && (
                    <motion.div 
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="space-y-3 overflow-hidden"
                    >
                      <div className="space-y-1">
                        <label className="text-[10px] font-bold text-slate-400 uppercase">Configuration</label>
                        <textarea 
                          className="input-field font-mono text-[10px] h-32 resize-none"
                          placeholder="Paste WireGuard config here..."
                          value={wgConfig.config || ''}
                          onChange={(e) => setWgConfig({...wgConfig, config: e.target.value})}
                        />
                      </div>
                      
                      {isWgChanged && (
                        <motion.button 
                          initial={{ opacity: 0, scale: 0.95 }}
                          animate={{ opacity: 1, scale: 1 }}
                          onClick={handleApplyWg}
                          className="btn-primary w-full text-xs py-1.5 shadow-md shadow-maroon/10"
                        >
                          Apply Changes
                        </motion.button>
                      )}
                    </motion.div>
                  )}
                </AnimatePresence>
                <AnimatePresence>
                  {isWgInfo && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="overflow-hidden"
                    >
                      <div className="pt-3 border-t border-slate-100">
                        <InterfaceDetails patterns={[/^wg\d/]} ifaces={ifaces} />
                      </div>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>}
            </div>
          </section>

        </aside>
      </main>

      <footer className="mt-auto pt-8 pb-4 border-t border-slate-200 flex justify-between items-center">
        <div className="flex items-center gap-2 text-slate-400">
          <Cpu className="w-4 h-4" />
          <span className="text-xs font-medium">Firmware: {appVersion}</span>
        </div>
        <div className="flex items-center gap-5 text-[10px] font-bold text-slate-400 uppercase tracking-wider">
          <span>CPU: {metrics.cpu}%</span>
          <span>RAM: {metrics.ram}%</span>
          <span>Uptime: {metrics.uptime}</span>
        </div>
      </footer>
    </div>
  );
}
