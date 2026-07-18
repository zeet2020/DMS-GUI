import {useEffect, useRef, useState} from 'react';
import {
    Accordion,
    ActionIcon,
    Badge,
    Button,
    Checkbox,
    Code,
    Collapse,
    Container,
    Divider,
    Group,
    Paper,
    ScrollArea,
    Select,
    Stack,
    Text,
    TextInput,
    Title,
    Tooltip,
    UnstyledButton,
} from '@mantine/core';
import {
    IconAdjustmentsHorizontal,
    IconChevronRight,
    IconFolderOpen,
    IconPhoto,
    IconPlayerPlayFilled,
    IconPlayerStopFilled,
    IconServer2,
    IconTrash,
} from '@tabler/icons-react';
import {Events, Window} from '@wailsio/runtime';
import {
    CheckTools,
    IsAutoLaunchEnabled,
    IsRunning,
    ListInterfaces,
    LoadConfig,
    SaveConfig,
    SelectDirectory,
    SelectIconFile,
    ServerInfo,
    SetAutoLaunch,
    StartServer,
    StopServer,
} from '../bindings/dms-gui/app';
import {Config, Tools} from '../bindings/dms-gui/models';

const AUTO_IFACE = '__auto__';
const MAX_LOG_LINES = 2000;
const WINDOW_WIDTH = 820;

// dms only supports these forceTranscodeTo targets (see -forceTranscodeTo).
const TRANSCODE_TARGETS = [
    {value: '', label: 'Off (no forced transcode)'},
    {value: 'chromecast', label: 'Chromecast'},
    {value: 'vp8', label: 'VP8'},
    {value: 'web', label: 'Web'},
];

const EMPTY_CONFIG = Config.createFrom({
    path: '',
    httpPort: ':1338',
    friendlyName: '',
    ifname: '',
    allowedIps: '',
    deviceIcon: '',
    ffprobeCachePath: '',
    forceTranscodeTo: '',
    ignoreHidden: false,
    ignoreUnreadable: false,
    noProbe: false,
    noTranscode: false,
    autoStartServer: false,
});

function App() {
    const [cfg, setCfg] = useState<Config>(EMPTY_CONFIG);
    const [iface, setIface] = useState<string>(AUTO_IFACE);
    const [interfaces, setInterfaces] = useState<string[]>([]);
    const [running, setRunning] = useState(false);
    const [busy, setBusy] = useState(false);
    const [info, setInfo] = useState('');
    const [logs, setLogs] = useState<string[]>([]);
    const [logOpen, setLogOpen] = useState(false);
    const [tools, setTools] = useState<Tools>(new Tools({ffmpeg: true, ffprobe: true}));
    const [loaded, setLoaded] = useState(false);
    const [autoLaunch, setAutoLaunch] = useState(false);

    const viewportRef = useRef<HTMLDivElement>(null);
    const measureRef = useRef<HTMLDivElement>(null);
    const autoStartedRef = useRef(false);

    const appendLog = (line: string) =>
        setLogs((prev) => {
            const next = [...prev, line];
            return next.length > MAX_LOG_LINES ? next.slice(next.length - MAX_LOG_LINES) : next;
        });

    const setField = <K extends keyof Config>(key: K, value: Config[K]) =>
        setCfg((prev) => Config.createFrom({...prev, [key]: value}));

    // Restore the saved config, interfaces, version and live state on mount.
    // Tool detection still forces the probe/transcode toggles when ffprobe/ffmpeg
    // are unavailable, even if the saved config had them off. A stale saved
    // interface (renamed/removed since last run) falls back to Auto.
    useEffect(() => {
        Promise.all([LoadConfig(), CheckTools(), ListInterfaces()]).then(([saved, t, ifs]) => {
            setTools(t);
            setInterfaces(ifs);
            setCfg(Config.createFrom({
                ...saved,
                noProbe: saved.noProbe || !t.ffprobe,
                noTranscode: saved.noTranscode || !t.ffmpeg,
            }));
            setIface(saved.ifname && ifs.includes(saved.ifname) ? saved.ifname : AUTO_IFACE);
            setLoaded(true);
        });
        ServerInfo().then(setInfo);
        IsRunning().then(setRunning);
        IsAutoLaunchEnabled().then(setAutoLaunch);

        const offLog = Events.On('dms:log', (e: any) => appendLog(e.data));
        const offStatus = Events.On('dms:status', (e: any) => {
            const s = e.data as {running: boolean; message: string};
            setRunning(s.running);
            appendLog(`>>> ${s.message}`);
        });

        return () => {
            offLog();
            offStatus();
        };
    }, []);

    // Persist the form on every control change, debounced so rapid typing does
    // not thrash the disk. Skipped until the initial load completes so we never
    // overwrite the saved file with empty/default values.
    useEffect(() => {
        if (!loaded) return;
        const payload = Config.createFrom({
            ...cfg,
            ifname: iface === AUTO_IFACE ? '' : iface,
        });
        const id = setTimeout(() => {
            SaveConfig(payload);
        }, 500);
        return () => clearTimeout(id);
    }, [cfg, iface, loaded]);

    // Match the window height to the actual rendered content so there is no
    // empty space when the log is collapsed and no clipping when it expands.
    useEffect(() => {
        const el = measureRef.current;
        if (!el || !('ResizeObserver' in window)) return;
        const fit = () => Window.SetSize(WINDOW_WIDTH, Math.ceil(el.getBoundingClientRect().height));
        const ro = new ResizeObserver(fit);
        ro.observe(el);
        fit();
        return () => ro.disconnect();
    }, []);

    // Keep the log pinned to the latest line while it is open.
    useEffect(() => {
        if (!logOpen) return;
        const vp = viewportRef.current;
        if (vp) vp.scrollTo({top: vp.scrollHeight});
    }, [logs, logOpen]);

    const browse = async () => {
        const dir = await SelectDirectory();
        if (dir) setField('path', dir);
    };

    const browseIcon = async () => {
        const file = await SelectIconFile();
        if (file) setField('deviceIcon', file);
    };

    const start = async () => {
        setBusy(true);
        try {
            const payload = Config.createFrom({
                ...cfg,
                ifname: iface === AUTO_IFACE ? '' : iface,
            });
            await StartServer(payload);
        } catch (e: any) {
            appendLog(`>>> ERROR: ${e}`);
        } finally {
            setBusy(false);
        }
    };

    const stop = async () => {
        setBusy(true);
        try {
            await StopServer();
        } catch (e: any) {
            appendLog(`>>> ERROR: ${e}`);
        } finally {
            setBusy(false);
        }
    };

    // When the saved config asks for it, start the server once after the initial
    // load resolves — so an app opened at login (or reopened) begins serving
    // without a click. The ref guard keeps it to a single attempt.
    useEffect(() => {
        if (!loaded || autoStartedRef.current) return;
        if (cfg.autoStartServer && cfg.path && !running) {
            autoStartedRef.current = true;
            start();
        }
    }, [loaded, cfg.autoStartServer, cfg.path, running]);

    const toggleAutoLaunch = async (enable: boolean) => {
        try {
            await SetAutoLaunch(enable);
        } catch (e: any) {
            appendLog(`>>> ERROR: ${e}`);
        }
        // Reflect the real OS state, not the optimistic checkbox value.
        IsAutoLaunchEnabled().then(setAutoLaunch);
    };

    const ifaceData = [
        {value: AUTO_IFACE, label: 'Auto (all interfaces)'},
        ...interfaces.map((n) => ({value: n, label: n})),
    ];

    const canStart = !running && !busy && cfg.path !== '';

    return (
        <Container size="md" py="md" ref={measureRef}>
            <Group justify="space-between" align="center" mb="sm">
                <Group gap="xs">
                    <IconServer2 size={28}/>
                    <Title order={3}>DMS</Title>
                    <Badge color={running ? 'teal' : 'gray'} variant={running ? 'filled' : 'light'}>
                        {running ? 'Running' : 'Stopped'}
                    </Badge>
                </Group>
                <Text size="xs" c="dimmed">{info}</Text>
            </Group>

            <Paper withBorder shadow="xs" p="md" radius="md">
                <Stack gap="sm">
                    <Group align="flex-end" gap="xs" wrap="nowrap">
                        <TextInput
                            label="Media folder to share"
                            placeholder="/path/to/media"
                            value={cfg.path}
                            onChange={(e) => setField('path', e.currentTarget.value)}
                            style={{flex: 1}}
                        />
                        <Button
                            variant="default"
                            leftSection={<IconFolderOpen size={16}/>}
                            onClick={browse}
                            disabled={running}
                        >
                            Browse
                        </Button>
                    </Group>

                    <Group grow align="flex-start">
                        <TextInput
                            label="HTTP port"
                            placeholder=":1338"
                            value={cfg.httpPort}
                            onChange={(e) => setField('httpPort', e.currentTarget.value)}
                            disabled={running}
                        />
                        <TextInput
                            label="Friendly name"
                            placeholder="auto (hostname)"
                            value={cfg.friendlyName}
                            onChange={(e) => setField('friendlyName', e.currentTarget.value)}
                            disabled={running}
                        />
                    </Group>

                    <Group grow align="flex-start">
                        <Select
                            label="Network interface (SSDP)"
                            data={ifaceData}
                            value={iface}
                            onChange={(v) => setIface(v ?? AUTO_IFACE)}
                            disabled={running}
                            allowDeselect={false}
                        />
                        <TextInput
                            label="Allowed client IPs"
                            placeholder="comma separated, blank = all"
                            value={cfg.allowedIps}
                            onChange={(e) => setField('allowedIps', e.currentTarget.value)}
                            disabled={running}
                        />
                    </Group>

                    <Divider label="Options" labelPosition="left"/>

                    <Group gap="lg">
                        <Checkbox
                            label="Ignore hidden"
                            checked={cfg.ignoreHidden}
                            onChange={(e) => setField('ignoreHidden', e.currentTarget.checked)}
                            disabled={running}
                        />
                        <Checkbox
                            label="Ignore unreadable"
                            checked={cfg.ignoreUnreadable}
                            onChange={(e) => setField('ignoreUnreadable', e.currentTarget.checked)}
                            disabled={running}
                        />
                        <Tooltip label="ffprobe is not installed — probing unavailable" disabled={tools.ffprobe}>
                            <Checkbox
                                label="No ffprobe"
                                checked={!tools.ffprobe || cfg.noProbe}
                                onChange={(e) => setField('noProbe', e.currentTarget.checked)}
                                disabled={running || !tools.ffprobe}
                            />
                        </Tooltip>
                        <Tooltip label="ffmpeg is not installed — transcoding unavailable" disabled={tools.ffmpeg}>
                            <Checkbox
                                label="No transcode"
                                checked={!tools.ffmpeg || cfg.noTranscode}
                                onChange={(e) => setField('noTranscode', e.currentTarget.checked)}
                                disabled={running || !tools.ffmpeg}
                            />
                        </Tooltip>
                    </Group>

                    <Accordion variant="separated" chevronPosition="left" radius="md">
                        <Accordion.Item value="advanced">
                            <Accordion.Control icon={<IconAdjustmentsHorizontal size={18}/>}>
                                <Text size="sm" fw={600}>Additional options</Text>
                            </Accordion.Control>
                            <Accordion.Panel>
                                <Stack gap="sm">
                                    <Group align="flex-end" gap="xs" wrap="nowrap">
                                        <TextInput
                                            label="Device icon (optional)"
                                            placeholder="PNG shown in DLNA clients"
                                            value={cfg.deviceIcon}
                                            onChange={(e) => setField('deviceIcon', e.currentTarget.value)}
                                            disabled={running}
                                            style={{flex: 1}}
                                        />
                                        <Button
                                            variant="default"
                                            leftSection={<IconPhoto size={16}/>}
                                            onClick={browseIcon}
                                            disabled={running}
                                        >
                                            Browse
                                        </Button>
                                        {cfg.deviceIcon && (
                                            <Button
                                                variant="subtle"
                                                color="gray"
                                                onClick={() => setField('deviceIcon', '')}
                                                disabled={running}
                                            >
                                                Clear
                                            </Button>
                                        )}
                                    </Group>

                                    <TextInput
                                        label="FFprobe cache file"
                                        description="Persists ffprobe results between runs (-fFprobeCachePath)"
                                        placeholder="~/.dms-ffprobe-cache"
                                        value={cfg.ffprobeCachePath}
                                        onChange={(e) => setField('ffprobeCachePath', e.currentTarget.value)}
                                        disabled={running || cfg.noProbe}
                                    />

                                    <Tooltip
                                        label="Uncheck 'No transcode' to force a target"
                                        disabled={tools.ffmpeg && !cfg.noTranscode}
                                    >
                                        <Select
                                            label="Force transcode to"
                                            description="Force all media to one profile (-forceTranscodeTo)"
                                            data={TRANSCODE_TARGETS}
                                            value={cfg.forceTranscodeTo ?? ''}
                                            onChange={(v) => setField('forceTranscodeTo', v ?? '')}
                                            disabled={running || cfg.noTranscode || !tools.ffmpeg}
                                            allowDeselect={false}
                                        />
                                    </Tooltip>

                                    <Divider label="Startup" labelPosition="left"/>

                                    <Checkbox
                                        label="Launch DMS on system startup"
                                        description="Open the app automatically when you log in"
                                        checked={autoLaunch}
                                        onChange={(e) => toggleAutoLaunch(e.currentTarget.checked)}
                                    />
                                    <Checkbox
                                        label="Start server automatically on launch"
                                        description="Begin serving as soon as the app opens (needs a media folder)"
                                        checked={cfg.autoStartServer}
                                        onChange={(e) => setField('autoStartServer', e.currentTarget.checked)}
                                        disabled={running}
                                    />
                                </Stack>
                            </Accordion.Panel>
                        </Accordion.Item>
                    </Accordion>

                    <Group justify="space-between" mt="xs">
                        <Group gap="xs">
                            <Button
                                color="teal"
                                leftSection={<IconPlayerPlayFilled size={16}/>}
                                onClick={start}
                                disabled={!canStart}
                                loading={busy && !running}
                            >
                                Start
                            </Button>
                            <Button
                                color="red"
                                variant="light"
                                leftSection={<IconPlayerStopFilled size={16}/>}
                                onClick={stop}
                                disabled={!running || busy}
                            >
                                Stop
                            </Button>
                        </Group>
                    </Group>
                </Stack>
            </Paper>

            <Group justify="space-between" align="center" mt="md" mb="xs">
                <UnstyledButton onClick={() => setLogOpen((o) => !o)}>
                    <Group gap={6}>
                        <IconChevronRight
                            size={16}
                            style={{transform: logOpen ? 'rotate(90deg)' : 'none', transition: 'transform 150ms'}}
                        />
                        <Text fw={600} size="sm">Server log</Text>
                        {logs.length > 0 && (
                            <Badge size="xs" variant="light" color="gray">{logs.length}</Badge>
                        )}
                    </Group>
                </UnstyledButton>
                <Tooltip label="Clear log">
                    <ActionIcon variant="subtle" color="gray" onClick={() => setLogs([])}>
                        <IconTrash size={16}/>
                    </ActionIcon>
                </Tooltip>
            </Group>

            <Collapse expanded={logOpen}>
                <Paper withBorder radius="md" style={{height: 320, overflow: 'hidden'}}>
                    <ScrollArea h="100%" viewportRef={viewportRef} className="dms-log">
                        <Code block style={{background: 'transparent', fontSize: 12, whiteSpace: 'pre-wrap'}}>
                            {logs.length ? logs.join('\n') : 'No output yet. Pick a folder and start the server.'}
                        </Code>
                    </ScrollArea>
                </Paper>
            </Collapse>
        </Container>
    );
}

export default App
