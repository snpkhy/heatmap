import ArrowBackRoundedIcon from "@mui/icons-material/ArrowBackRounded";
import PauseRoundedIcon from "@mui/icons-material/PauseRounded";
import PlayArrowRoundedIcon from "@mui/icons-material/PlayArrowRounded";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import FormControlLabel from "@mui/material/FormControlLabel";
import IconButton from "@mui/material/IconButton";
import LinearProgress from "@mui/material/LinearProgress";
import Link from "@mui/material/Link";
import MenuItem from "@mui/material/MenuItem";
import Slider from "@mui/material/Slider";
import Stack from "@mui/material/Stack";
import Switch from "@mui/material/Switch";
import Tab from "@mui/material/Tab";
import Tabs from "@mui/material/Tabs";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import { BarChart } from "@mui/x-charts/BarChart";
import { LineChart } from "@mui/x-charts/LineChart";
import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import {
  api,
  type DirectionHistogram,
  eventsURL,
  type PipelineInfo,
  type Progress,
  type Run,
  type RunDetail,
  runFile,
} from "../api";
import { type LayerMode, type Layers, STAGE_BLACK, Stage } from "../components/Stage";
import { Timeline } from "../components/Timeline";
import {
  compass,
  dateTime,
  decibels,
  fps,
  metric,
  noiseLabel,
  percent,
  stageLabel,
} from "../format";
import { HeatEngine, PALETTES, paletteCSS } from "../heat/engine";
import { FrameStore } from "../heat/frames";
import { useT } from "../i18n";
import { useSeriesColors } from "../palette";
import { navigate, routeHref } from "../router";
import { fonts } from "../theme";
import { statusColor } from "./History";
import { madeWith } from "./Scenes";

/** Frames per second at 1× speed. */
const BASE_FPS = 24;
const SPEEDS = [0.5, 1, 2, 4];

const defaultLayers: Layers = {
  mode: "trails",
  palette: "inferno",
  opacity: 0.9,
  memory: 28,
  footage: true,
  outline: false,
};

/** What can be played: which frames a run has and how large they are. */
interface Reel {
  start: number;
  total: number;
  width: number;
  height: number;
}

function reelOf(detail: RunDetail | null): Reel | null {
  const run = detail?.run;
  if (run) {
    if (!run.saved_frames) return null;
    const s = run.summary;
    return { start: s.start, total: s.frames, width: s.width, height: s.height };
  }
  const live = detail?.live;
  const total = detail?.progress?.total ?? 0;
  if (!live || total <= 0 || (detail?.status !== "running" && detail?.status !== "queued")) {
    return null;
  }
  return { start: live.start, total, width: live.scene.width, height: live.scene.height };
}

export function Viewer({ id }: { id: string }) {
  const t = useT();
  const [detail, setDetail] = useState<RunDetail | null>(null);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    api
      .run(id)
      .then((d) => {
        setDetail(d);
        if (d.progress) setProgress(d.progress);
      })
      .catch((e: Error) => setError(e.message));
  }, [id]);

  useEffect(load, [load]);

  const active = detail?.status === "queued" || detail?.status === "running";
  // Live progress: Server-Sent Events, or polling where EventSource is missing.
  useEffect(() => {
    if (!active) return;
    if (typeof EventSource === "undefined") {
      const timer = setInterval(load, 1000);
      return () => clearInterval(timer);
    }
    const es = new EventSource(eventsURL(id));
    es.addEventListener("progress", (e) => setProgress(JSON.parse((e as MessageEvent).data)));
    es.addEventListener("done", () => {
      es.close();
      load();
    });
    es.onerror = () => {
      es.close();
      setTimeout(load, 1000);
    };
    return () => es.close();
  }, [active, id, load]);

  if (error) return <Alert severity="error">{error}</Alert>;
  if (!detail) return <LinearProgress />;

  const run = detail.run;
  const sequence = run?.summary.sequence ?? detail.live?.scene.id ?? "";
  const noise = run ? noiseLabel(run.summary.noise, t.noise) : t.noise.none;
  return (
    <Stack spacing={2.5}>
      <Box>
        <Link
          href={routeHref({ page: "launch" })}
          underline="hover"
          color="text.secondary"
          sx={{ display: "inline-flex", alignItems: "center", gap: 0.5, fontSize: 14 }}
        >
          <ArrowBackRoundedIcon sx={{ fontSize: 16 }} /> {t.nav.scenes}
        </Link>
        <Stack direction="row" spacing={2} sx={{ alignItems: "center", flexWrap: "wrap", mt: 0.5 }}>
          <Typography variant="h2" component="h2" sx={{ fontSize: { xs: 30, md: 38 } }}>
            {sequence ? t.sceneTitle(sequence) : t.viewer.analysis}
          </Typography>
          {detail.status !== "done" && (
            <Chip size="small" color={statusColor[detail.status]} label={t.status[detail.status]} />
          )}
          <Box sx={{ flexGrow: 1 }} />
          {run && <ModeSwitch run={run} />}
        </Stack>
        {run && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.75 }}>
            {t.modeName(run.summary.pipeline, run.summary.pipeline_name)} ·{" "}
            {t.viewer.frameRange(run.summary.start, run.summary.start + run.summary.frames - 1)}
            {noise !== t.noise.none && ` · ${t.viewer.withNoise(noise)}`} ·{" "}
            {dateTime(run.summary.created_at, t.locale)}
          </Typography>
        )}
      </Box>
      {detail.error && <Alert severity="error">{detail.error}</Alert>}
      <Player id={id} detail={detail} progress={progress} />
    </Stack>
  );
}

/**
 * Shows the same footage through another detection mode: it opens the
 * analysis of these frames made with that mode, or starts one.
 */
function ModeSwitch({ run }: { run: Run }) {
  const t = useT();
  const [modes, setModes] = useState<PipelineInfo[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const s = run.summary;

  useEffect(() => {
    api
      .pipelines()
      .then((p) => setModes(p.filter((m) => !m.error)))
      .catch(() => {});
  }, []);

  const pick = async (mode: string) => {
    if (mode === s.pipeline) return;
    setBusy(true);
    setError("");
    const quiet = !s.noise.type || s.noise.type === "none" || s.noise.level === 0;
    const noiseType = quiet ? "none" : s.noise.type;
    try {
      const made = (await api.runs()).find(
        (r) =>
          r.status === "done" &&
          r.summary?.sequence === s.sequence &&
          r.summary.start === s.start &&
          r.summary.frames === s.frames &&
          madeWith(r.summary, mode, noiseType, s.noise.level),
      );
      const id =
        made?.run_id ??
        (
          await api.startRun({
            pipeline: mode,
            sequence: s.sequence,
            noise: quiet ? "none" : `${s.noise.type}:${s.noise.level}`,
            seed: s.noise.seed,
            start: s.start,
            frames: s.frames,
          })
        ).run_id;
      navigate({ page: "run", id });
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  };

  if (modes.length < 2) return null;
  return (
    <TextField
      select
      size="small"
      label={t.scenes.mode}
      value={modes.some((m) => m.id === s.pipeline) ? s.pipeline : ""}
      disabled={busy}
      error={!!error}
      helperText={error || undefined}
      onChange={(e) => pick(e.target.value)}
      sx={{ minWidth: 250 }}
    >
      {modes.map((m) => (
        <MenuItem key={m.id} value={m.id}>
          {t.modeName(m.id, m.name)}
        </MenuItem>
      ))}
    </TextField>
  );
}

function Player({
  id,
  detail,
  progress,
}: {
  id: string;
  detail: RunDetail;
  progress: Progress | null;
}) {
  const t = useT();
  const run = detail.run;
  const active = detail.status === "queued" || detail.status === "running";
  const reel = reelOf(detail);
  const reelKey = reel ? `${reel.start}:${reel.total}:${reel.width}:${reel.height}` : "";

  const [version, setVersion] = useState(0);
  const [index, setIndex] = useState(0);
  const [playing, setPlaying] = useState(true);
  const [speed, setSpeed] = useState(1);
  const [layers, setLayers] = useState(defaultLayers);
  const set = <K extends keyof Layers>(key: K, value: Layers[K]) =>
    setLayers((l) => ({ ...l, [key]: value }));

  // The store survives the moment a live run finishes: the frames are the same.
  // biome-ignore lint/correctness/useExhaustiveDependencies: reelKey stands for the reel.
  const store = useMemo(
    () =>
      reel
        ? new FrameStore(
            (i, file) => runFile(id, `frames/${reel.start + i}/${file}`),
            reel.total,
            reel.width,
            reel.height,
            () => setVersion((v) => v + 1),
          )
        : null,
    [id, reelKey],
  );
  const engine = useMemo(() => (store ? new HeatEngine(store.gridW, store.gridH) : null), [store]);

  useEffect(() => {
    store?.resume();
    return () => store?.dispose();
  }, [store]);

  // Frames exist on the server once they are processed: all of them for a
  // finished run, the first `done` for a run in progress.
  const available = run ? (reel?.total ?? 0) : (progress?.done ?? 0);
  useEffect(() => {
    store?.setAvailable(available);
  }, [store, available]);

  // While a run is being processed the picture follows it; then it plays from the start.
  // biome-ignore lint/correctness/useExhaustiveDependencies: version says the store has more frames.
  useEffect(() => {
    if (active && store) setIndex(Math.max(0, store.ready - 1));
  }, [active, store, version]);
  useEffect(() => {
    if (!active) {
      setIndex(0);
      setPlaying(true);
    }
  }, [active]);

  useEffect(() => {
    if (!playing || active || !store || typeof requestAnimationFrame === "undefined") return;
    let raf = 0;
    let last = performance.now();
    let owed = 0;
    const tick = (now: number) => {
      owed += ((now - last) * speed * BASE_FPS) / 1000;
      last = now;
      const steps = Math.floor(owed);
      if (steps > 0) {
        owed -= steps;
        setIndex((i) => {
          const next = i + steps;
          if (next < store.ready) return next;
          // At the end of the footage start over; at the end of what is loaded, wait.
          return store.complete ? next % store.length : Math.max(0, store.ready - 1);
        });
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [playing, active, speed, store]);

  const seek = useCallback(
    (i: number) => {
      if (!store || store.ready === 0) return;
      setIndex(Math.max(0, Math.min(i, store.ready - 1)));
    },
    [store],
  );

  // Space plays and pauses, the arrow keys step frame by frame.
  useEffect(() => {
    if (!store || active) return;
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      if (el?.closest?.("input, textarea, select, button, a, [role='slider'], [role='tab']"))
        return;
      if (e.key === " ") {
        e.preventDefault();
        setPlaying((p) => !p);
      } else if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
        e.preventDefault();
        setPlaying(false);
        const step = e.key === "ArrowRight" ? 1 : -1;
        setIndex((i) => Math.max(0, Math.min(i + step, store.ready - 1)));
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [store, active]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: version says the store has more frames.
  const busiest = useMemo(() => {
    if (!store) return -1;
    let best = -1;
    for (let i = 0; i < store.ready; i++) {
      if (best < 0 || store.activity[i] > store.activity[best]) best = i;
    }
    return best;
  }, [store, version]);

  if (!store || !engine || !reel) {
    if (run) return <StillResult id={id} run={run} />;
    if (active) {
      return (
        <Panel>
          <Typography color="text.secondary" gutterBottom>
            {t.viewer.queue}
          </Typography>
          <LinearProgress aria-label={t.viewer.runProgress} />
        </Panel>
      );
    }
    return null;
  }

  const shown = Math.min(index, Math.max(0, store.ready - 1));
  const frameNumber = reel.start + shown;
  const heat = layers.mode !== "footage";
  const done = progress?.done ?? 0;
  const loadedPct = (100 * store.ready) / store.length;
  const digits = String(reel.start + reel.total - 1).length;

  return (
    <>
      <Box
        sx={{
          display: "grid",
          gap: 3,
          alignItems: "start",
          gridTemplateColumns: { xs: "minmax(0, 1fr)", lg: "minmax(0, 1fr) 300px" },
        }}
      >
        <Stack spacing={2} sx={{ minWidth: 0 }}>
          <Stage
            store={store}
            engine={engine}
            index={shown}
            layers={layers}
            version={version}
            width={reel.width}
            height={reel.height}
            label={t.viewer.stage}
          >
            {active && (
              <Stack
                direction="row"
                spacing={1}
                sx={{
                  position: "absolute",
                  top: 14,
                  left: 14,
                  alignItems: "center",
                  px: 1.25,
                  py: 0.5,
                  borderRadius: "999px",
                  bgcolor: "rgba(7,6,10,0.72)",
                  color: "#fff",
                  backdropFilter: "blur(6px)",
                }}
              >
                <Box
                  sx={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    bgcolor: "#ff6b61",
                    animation: "pulse 1.2s ease-in-out infinite",
                    "@keyframes pulse": { "50%": { opacity: 0.25 } },
                  }}
                />
                <Typography variant="caption" sx={{ fontFamily: fonts.mono, fontWeight: 600 }}>
                  {progress && progress.total > 0
                    ? t.viewer.live(done, progress.total, fps(progress.fps))
                    : t.viewer.queue}
                </Typography>
              </Stack>
            )}
            {heat && (
              <Stack
                direction="row"
                spacing={1}
                sx={{
                  position: "absolute",
                  left: 14,
                  bottom: 14,
                  alignItems: "center",
                  px: 1.25,
                  py: 0.6,
                  borderRadius: "999px",
                  bgcolor: "rgba(7,6,10,0.72)",
                  color: "rgba(255,255,255,0.86)",
                  backdropFilter: "blur(6px)",
                }}
              >
                <Typography variant="caption">{t.viewer.quiet}</Typography>
                <Box
                  sx={{
                    width: 96,
                    height: 6,
                    borderRadius: "3px",
                    background: paletteCSS(layers.palette),
                  }}
                />
                <Typography variant="caption">{t.viewer.busy}</Typography>
              </Stack>
            )}
            {(active || !store.complete) && (
              <LinearProgress
                variant="determinate"
                aria-label={active ? t.viewer.runProgress : t.viewer.loadingFrames}
                value={
                  active
                    ? progress && progress.total > 0
                      ? (100 * done) / progress.total
                      : 0
                    : loadedPct
                }
                sx={{ position: "absolute", left: 0, right: 0, bottom: 0, height: 3 }}
              />
            )}
          </Stage>

          <Stack direction="row" spacing={1.5} sx={{ alignItems: "center" }}>
            <IconButton
              aria-label={playing && !active ? t.viewer.pause : t.viewer.play}
              disabled={active}
              onClick={() => setPlaying((p) => !p)}
              sx={{
                width: 48,
                height: 48,
                flex: "none",
                bgcolor: "primary.main",
                color: "primary.contrastText",
                "&:hover": { bgcolor: "primary.dark" },
                "&.Mui-disabled": { bgcolor: "action.disabledBackground" },
              }}
            >
              {playing && !active ? <PauseRoundedIcon /> : <PlayArrowRoundedIcon />}
            </IconButton>
            <Timeline
              activity={store.activity}
              ready={store.ready}
              index={shown}
              label={t.viewer.frameSlider}
              disabled={active}
              onSeek={(i) => {
                setPlaying(false);
                seek(i);
              }}
            />
            <Typography
              sx={{ fontFamily: fonts.mono, fontSize: 13, whiteSpace: "nowrap", flex: "none" }}
              color="text.secondary"
            >
              <Box component="span" sx={{ color: "text.primary" }}>
                {String(frameNumber).padStart(digits, "0")}
              </Box>{" "}
              / {reel.start + reel.total - 1}
            </Typography>
            <Tooltip title={t.viewer.speed}>
              <Button
                size="small"
                color="inherit"
                aria-label={t.viewer.speedLabel}
                onClick={() => setSpeed((s) => SPEEDS[(SPEEDS.indexOf(s) + 1) % SPEEDS.length])}
                sx={{ minWidth: 52, fontFamily: fonts.mono, flex: "none" }}
              >
                {speed}×
              </Button>
            </Tooltip>
          </Stack>

          <Panel>
            <Stack
              direction={{ xs: "column", md: "row" }}
              spacing={{ xs: 2, md: 3 }}
              sx={{ alignItems: { md: "center" }, flexWrap: "wrap" }}
              useFlexGap
            >
              <ToggleButtonGroup
                exclusive
                size="small"
                color="primary"
                aria-label={t.viewer.view}
                value={layers.mode}
                onChange={(_, v: LayerMode | null) => v && set("mode", v)}
              >
                <ToggleButton value="trails">{t.viewer.trails}</ToggleButton>
                <ToggleButton value="heatmap">{t.viewer.heatmap}</ToggleButton>
                <ToggleButton value="footage">{t.viewer.videoOnly}</ToggleButton>
              </ToggleButtonGroup>

              <Stack direction="row" spacing={1} aria-label={t.viewer.palette} role="group">
                {PALETTES.map((p) => (
                  <Tooltip key={p.id} title={t.viewer.palettes[p.id]}>
                    <Box
                      component="button"
                      type="button"
                      aria-label={t.viewer.paletteLabel(t.viewer.palettes[p.id])}
                      aria-pressed={layers.palette === p.id}
                      disabled={!heat}
                      onClick={() => set("palette", p.id)}
                      sx={{
                        width: 30,
                        height: 30,
                        p: 0,
                        borderRadius: "50%",
                        cursor: "pointer",
                        background: paletteCSS(p.id, "135deg"),
                        border: "2px solid",
                        borderColor: "background.paper",
                        outline: "2px solid",
                        outlineColor: layers.palette === p.id ? "primary.main" : "transparent",
                        transition: "outline-color .15s, opacity .15s",
                        "&:disabled": { opacity: 0.35, cursor: "default" },
                      }}
                    />
                  </Tooltip>
                ))}
              </Stack>

              <Dial label={t.viewer.intensity} disabled={!heat}>
                <Slider
                  size="small"
                  aria-label={t.viewer.intensityLabel}
                  disabled={!heat}
                  value={layers.opacity}
                  min={0.2}
                  max={1}
                  step={0.05}
                  onChange={(_, v) => set("opacity", v as number)}
                />
              </Dial>
              <Dial label={t.viewer.trailLength} disabled={layers.mode !== "trails"}>
                <Slider
                  size="small"
                  aria-label={t.viewer.trailLengthLabel}
                  disabled={layers.mode !== "trails"}
                  value={layers.memory}
                  min={4}
                  max={96}
                  step={2}
                  onChange={(_, v) => set("memory", v as number)}
                />
              </Dial>

              <Stack direction="row" spacing={1}>
                <FormControlLabel
                  label={t.viewer.thermal}
                  disabled={!heat}
                  control={
                    <Switch
                      size="small"
                      checked={!layers.footage}
                      onChange={(e) => set("footage", !e.target.checked)}
                    />
                  }
                  slotProps={{ typography: { variant: "body2" } }}
                />
                <FormControlLabel
                  label={t.viewer.outline}
                  control={
                    <Switch
                      size="small"
                      checked={layers.outline}
                      onChange={(e) => set("outline", e.target.checked)}
                    />
                  }
                  slotProps={{ typography: { variant: "body2" } }}
                />
              </Stack>
            </Stack>
          </Panel>
        </Stack>

        <Stack spacing={1.5}>
          <Box
            sx={{
              display: "grid",
              gap: 1.5,
              gridTemplateColumns: { xs: "repeat(2, 1fr)", sm: "repeat(4, 1fr)", lg: "1fr" },
            }}
          >
            <Stat
              label={t.viewer.movingNow}
              value={percent(store.ready > 0 ? store.activity[shown] : null)}
              note={t.viewer.movingNowNote}
            />
            <Stat
              label={t.viewer.covered}
              value={percent(store.ready > 0 ? store.coverage[shown] : null)}
              note={t.viewer.coveredNote}
            />
            <Stat
              label={t.viewer.busiest}
              value={busiest >= 0 ? t.frame(reel.start + busiest) : "—"}
              note={
                busiest >= 0 && !active ? (
                  <Link
                    component="button"
                    type="button"
                    onClick={() => {
                      setPlaying(false);
                      seek(busiest);
                    }}
                  >
                    {t.viewer.jump}
                  </Link>
                ) : undefined
              }
            />
            {run && <Summary run={run} />}
          </Box>
          {run?.summary.has_plan && (
            <Panel pad={0}>
              <Box
                component="img"
                src={runFile(id, "plan_heatmap.png")}
                alt={t.viewer.planAlt}
                sx={{ width: "100%", display: "block", bgcolor: STAGE_BLACK }}
              />
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ display: "block", p: 1.5 }}
              >
                {t.viewer.planNote}
              </Typography>
            </Panel>
          )}
        </Stack>
      </Box>

      {run && <Details id={id} run={run} frame={playing ? Math.floor(shown / 12) * 12 : shown} />}
    </>
  );
}

function Panel({ children, pad = 2 }: { children: ReactNode; pad?: number }) {
  return (
    <Box
      sx={{
        p: pad,
        borderRadius: "16px",
        border: 1,
        borderColor: "divider",
        bgcolor: "background.paper",
        overflow: "hidden",
      }}
    >
      {children}
    </Box>
  );
}

function Dial({
  label,
  disabled,
  children,
}: {
  label: string;
  disabled?: boolean;
  children: ReactNode;
}) {
  return (
    <Box sx={{ width: { xs: "100%", md: 132 }, opacity: disabled ? 0.45 : 1 }}>
      <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: -0.5 }}>
        {label}
      </Typography>
      {children}
    </Box>
  );
}

function Stat({ label, value, note }: { label: string; value: string; note?: ReactNode }) {
  return (
    <Panel>
      <Typography variant="overline" color="text.secondary" sx={{ display: "block" }}>
        {label}
      </Typography>
      <Typography
        component="p"
        sx={{
          fontFamily: fonts.display,
          fontSize: 30,
          fontWeight: 600,
          letterSpacing: "-0.03em",
          lineHeight: 1.2,
          whiteSpace: "nowrap",
        }}
      >
        {value}
      </Typography>
      {note && (
        <Typography variant="caption" color="text.secondary" component="div">
          {note}
        </Typography>
      )}
    </Panel>
  );
}

function directionSummary(run: Run): DirectionHistogram | undefined {
  return Object.values(run.summary.extra ?? {}).find(
    (v): v is DirectionHistogram => (v as DirectionHistogram).type === "direction_histogram",
  );
}

/** What is known once a run has finished: direction, accuracy, speed. */
function Summary({ run }: { run: Run }) {
  const t = useT();
  const s = run.summary;
  const dir = directionSummary(run);
  return (
    <>
      {dir?.dominant_deg !== undefined && (
        <Stat
          label={t.viewer.direction}
          value={compass(dir.dominant_deg, t.compass)}
          note={t.viewer.directionNote(dir.vectors)}
        />
      )}
      {s.f1 !== null && (
        <Stat
          label={t.viewer.accuracy}
          value={metric(s.f1)}
          note={t.viewer.accuracyNote(s.eval_frames, metric(s.precision), metric(s.recall))}
        />
      )}
      <Stat
        label={t.viewer.speedStat}
        value={`${fps(s.fps)} ${t.units.fps}`}
        note={t.viewer.speedNote(s.total_ms.toFixed(0), s.frames)}
      />
      {s.psnr !== null && (
        <Stat
          label={t.viewer.denoise}
          value={decibels(s.psnr, t.units.db)}
          note={`PSNR · SSIM ${metric(s.ssim)}`}
        />
      )}
    </>
  );
}

/** A finished run that was saved without frames: only its final pictures exist. */
function StillResult({ id, run }: { id: string; run: Run }) {
  const t = useT();
  return (
    <Stack spacing={2}>
      <Alert severity="info">{t.viewer.noFrames}</Alert>
      <Box
        sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "repeat(2, 1fr)" } }}
      >
        <Picture src={runFile(id, "overlay.png")} label={t.viewer.overlay} />
        <Picture src={runFile(id, "heatmap.png")} label={t.viewer.heatmapPicture} />
      </Box>
      <Box
        sx={{
          display: "grid",
          gap: 1.5,
          gridTemplateColumns: { xs: "repeat(2, 1fr)", md: "repeat(4, 1fr)" },
        }}
      >
        <Summary run={run} />
      </Box>
      <Charts run={run} />
    </Stack>
  );
}

function Picture({ src, label }: { src: string; label: string }) {
  return (
    <Panel pad={0}>
      <Box
        component="img"
        src={src}
        alt={label}
        loading="lazy"
        sx={{ width: "100%", display: "block", bgcolor: STAGE_BLACK }}
      />
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ display: "block", px: 1.25, py: 0.75 }}
      >
        {label}
      </Typography>
    </Panel>
  );
}

function Details({ id, run, frame }: { id: string; run: Run; frame: number }) {
  const t = useT();
  const [tab, setTab] = useState(0);
  return (
    <Stack spacing={2}>
      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs value={tab} onChange={(_, v) => setTab(v)} aria-label={t.viewer.details}>
          <Tab label={t.viewer.how} />
          <Tab label={t.viewer.charts} />
        </Tabs>
      </Box>
      {tab === 0 && <Steps id={id} run={run} index={frame} />}
      {tab === 1 && <Charts run={run} />}
    </Stack>
  );
}

/** One frame at every step from the camera picture to the motion mask. */
function Steps({ id, run, index }: { id: string; run: Run; index: number }) {
  const t = useT();
  const f = run.frames[index];
  if (!f) return null;
  const pics: [string, string][] = [
    [runFile(id, `frames/${f.frame}/input.png`), t.viewer.input],
    ...run.stage_keys.map((k): [string, string] => [
      runFile(id, `frames/${f.frame}/${k}.png`),
      stageLabel(k),
    ]),
    [runFile(id, `frames/${f.frame}/mask.png`), t.viewer.mask],
  ];
  if (f.has_gt) pics.push([runFile(id, `frames/${f.frame}/gt.png`), t.viewer.truth]);
  return (
    <Stack spacing={1.5}>
      <Typography variant="body2" color="text.secondary">
        {t.viewer.stepByStep(f.frame, f.f1 === null ? null : metric(f.f1), f.time_ms.toFixed(1))}
      </Typography>
      <Box
        sx={{
          display: "grid",
          gap: 1.5,
          gridTemplateColumns: { xs: "repeat(2, 1fr)", sm: "repeat(3, 1fr)", md: "repeat(4, 1fr)" },
        }}
      >
        {pics.map(([src, label]) => (
          <Picture key={label} src={src} label={label} />
        ))}
      </Box>
    </Stack>
  );
}

function Charts({ run }: { run: Run }) {
  const t = useT();
  const colors = useSeriesColors();
  const frames = run.frames;
  const x = frames.map((f) => f.frame);
  const dir = directionSummary(run);
  const hasGT = frames.some((f) => f.has_gt);
  return (
    <Stack spacing={2}>
      {hasGT ? (
        <Panel>
          <Typography variant="subtitle1">{t.viewer.accuracyPerFrame}</Typography>
          <LineChart
            height={300}
            xAxis={[{ data: x, label: t.viewer.frameAxis, scaleType: "linear" }]}
            yAxis={[{ min: 0, max: 1 }]}
            series={[
              {
                data: frames.map((f) => f.f1),
                label: t.metrics.f1,
                color: colors[0],
                showMark: false,
              },
              {
                data: frames.map((f) => f.precision),
                label: t.metrics.precision,
                color: colors[1],
                showMark: false,
              },
              {
                data: frames.map((f) => f.recall),
                label: t.metrics.recall,
                color: colors[2],
                showMark: false,
              },
            ]}
            grid={{ horizontal: true }}
          />
        </Panel>
      ) : (
        <Alert severity="info">{t.viewer.noTruth}</Alert>
      )}
      <Box
        sx={{
          display: "grid",
          gap: 2,
          gridTemplateColumns: { xs: "1fr", md: dir ? "repeat(2, 1fr)" : "1fr" },
        }}
      >
        <Panel>
          <Typography variant="subtitle1">{t.viewer.timePerStep}</Typography>
          <BarChart
            height={260}
            layout="horizontal"
            yAxis={[
              {
                data: run.summary.stages.map((s) => stageLabel(s.key)),
                scaleType: "band",
                width: 150,
              },
            ]}
            series={[
              {
                data: run.summary.stages.map((s) => Number(s.mean_ms.toFixed(3))),
                label: t.viewer.msPerFrame,
                color: colors[0],
              },
            ]}
            hideLegend
            grid={{ vertical: true }}
          />
        </Panel>
        {dir && (
          <Panel>
            <Typography variant="subtitle1">{t.viewer.directions(dir.vectors)}</Typography>
            <BarChart
              height={260}
              xAxis={[
                { data: dir.bin_centers.map((d) => compass(d, t.compass)), scaleType: "band" },
              ]}
              series={[{ data: dir.bins, label: t.viewer.vectors, color: colors[0] }]}
              hideLegend
              grid={{ horizontal: true }}
            />
          </Panel>
        )}
      </Box>
    </Stack>
  );
}
