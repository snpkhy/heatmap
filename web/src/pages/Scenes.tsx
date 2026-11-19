import PlayArrowRoundedIcon from "@mui/icons-material/PlayArrowRounded";
import ReplayRoundedIcon from "@mui/icons-material/ReplayRounded";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import MenuItem from "@mui/material/MenuItem";
import Skeleton from "@mui/material/Skeleton";
import Slider from "@mui/material/Slider";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { useEffect, useMemo, useState } from "react";
import {
  api,
  type PipelineInfo,
  previewURL,
  type RunListItem,
  runFile,
  type SequenceInfo,
  type Summary,
} from "../api";
import { useT } from "../i18n";
import { navigate, routeHref } from "../router";
import { fonts } from "../theme";

/** The warm half of the heat scale: bright enough to read as text on any surface. */
const textHeat = "linear-gradient(90deg, #e2466a 0%, #f98c0a 60%, #f6c544 100%)";

type NoiseType = "none" | "gaussian" | "salt_pepper";

const noiseRange: Record<
  Exclude<NoiseType, "none">,
  { min: number; max: number; step: number; def: number }
> = {
  gaussian: { min: 1, max: 40, step: 1, def: 10 },
  salt_pepper: { min: 0.005, max: 0.1, step: 0.005, def: 0.02 },
};

const lengths = [150, 300, 500];

export function noiseString(type: NoiseType, level: number): string {
  return type === "none" ? "none" : `${type}:${level}`;
}

/** What an analysis is started with. It is remembered, so it survives a visit to the viewer. */
interface Settings {
  mode: string;
  frames: number;
  noiseType: NoiseType;
  noiseLevel: number;
}

const SETTINGS_KEY = "heatmap.settings";
const defaults: Settings = { mode: "", frames: 300, noiseType: "none", noiseLevel: 10 };

function loadSettings(): Settings {
  try {
    const saved = JSON.parse(localStorage.getItem(SETTINGS_KEY) ?? "{}") as Partial<Settings>;
    return {
      mode: typeof saved.mode === "string" ? saved.mode : defaults.mode,
      frames: lengths.includes(saved.frames as number) ? (saved.frames as number) : defaults.frames,
      noiseType: saved.noiseType && saved.noiseType in noiseRange ? saved.noiseType : "none",
      noiseLevel: typeof saved.noiseLevel === "number" ? saved.noiseLevel : defaults.noiseLevel,
    };
  } catch {
    return defaults;
  }
}

/** Whether a finished analysis was made with these settings (its length aside). */
export function madeWith(s: Summary, mode: string, noiseType: string, noiseLevel: number) {
  if (s.pipeline !== mode) return false;
  const quiet = !s.noise.type || s.noise.type === "none" || s.noise.level === 0;
  if (noiseType === "none") return quiet;
  return !quiet && s.noise.type === noiseType && s.noise.level === noiseLevel;
}

/** The frame shown on a scene card: the middle of the part worth watching. */
export function posterFrame(seq: SequenceInfo): number {
  return seq.has_gt ? Math.floor((seq.eval_start + seq.eval_end) / 2) : Math.floor(seq.frames / 2);
}

/** Real footage first, then the built-in scenes, longest first. */
export function sceneOrder(a: SequenceInfo, b: SequenceInfo): number {
  const real = (s: SequenceInfo) => (s.source === "synthetic" ? 1 : 0);
  return real(a) - real(b) || b.frames - a.frames || a.id.localeCompare(b.id);
}

export function Scenes() {
  const t = useT();
  const [modes, setModes] = useState<PipelineInfo[]>([]);
  const [scenes, setScenes] = useState<SequenceInfo[] | null>(null);
  const [runs, setRuns] = useState<RunListItem[]>([]);
  const [settings, setSettings] = useState(loadSettings);
  const { mode, frames, noiseType, noiseLevel } = settings;
  const change = (patch: Partial<Settings>) => setSettings((s) => ({ ...s, ...patch }));
  const [error, setError] = useState("");
  const [starting, setStarting] = useState("");

  useEffect(() => {
    Promise.all([api.pipelines(), api.sequences()])
      .then(([p, s]) => {
        setModes(p);
        setScenes([...s].sort(sceneOrder));
        // Keep the remembered mode if it still exists, else take the first working one.
        setSettings((s) => ({
          ...s,
          mode: p.some((x) => x.id === s.mode && !x.error)
            ? s.mode
            : (p.find((x) => !x.error)?.id ?? ""),
        }));
      })
      .catch((e: Error) => setError(e.message));
    // Past analyses only decorate the cards, so the gallery works without them.
    api
      .runs()
      .then(setRuns)
      .catch(() => {});
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
    } catch {
      // Not remembered, but still applied.
    }
  }, [settings]);

  // The newest finished analysis of every scene made with the settings on
  // screen. Change the mode and a card offers to analyse again, not an old result.
  const latest = useMemo(() => {
    const out = new Map<string, string>();
    for (const r of runs) {
      const s = r.summary;
      if (r.status !== "done" || !s || out.has(s.sequence)) continue;
      if (madeWith(s, mode, noiseType, noiseLevel)) out.set(s.sequence, r.run_id);
    }
    return out;
  }, [runs, mode, noiseType, noiseLevel]);

  const analyze = async (seq: SequenceInfo) => {
    setStarting(seq.id);
    setError("");
    try {
      const { run_id } = await api.startRun({
        pipeline: mode,
        sequence: seq.id,
        noise: noiseString(noiseType, noiseLevel),
        seed: 1,
        frames,
      });
      navigate({ page: "run", id: run_id });
    } catch (e) {
      setError((e as Error).message);
      setStarting("");
    }
  };

  return (
    <Stack spacing={{ xs: 4, md: 5 }}>
      <Box sx={{ maxWidth: 860 }}>
        <Typography variant="overline" color="primary">
          {t.scenes.eyebrow}
        </Typography>
        <Typography
          variant="h2"
          component="h2"
          sx={{ fontSize: { xs: 38, sm: 50, md: 60 }, mt: 0.75 }}
        >
          {t.scenes.headline[0]}
          <Box
            component="em"
            sx={{
              fontStyle: "normal",
              background: textHeat,
              backgroundClip: "text",
              WebkitBackgroundClip: "text",
              color: "transparent",
              pr: 0.25,
            }}
          >
            {t.scenes.headline[1]}
          </Box>
          {t.scenes.headline[2]}
        </Typography>
        <Typography
          color="text.secondary"
          sx={{ mt: 2, fontSize: { xs: 16, md: 18 }, maxWidth: 600 }}
        >
          {t.scenes.lead}
        </Typography>
      </Box>

      {error && <Alert severity="error">{error}</Alert>}

      <Stack spacing={2.5}>
        <Stack
          direction={{ xs: "column", md: "row" }}
          spacing={2}
          sx={{ alignItems: { md: "flex-end" }, justifyContent: "space-between" }}
        >
          <Typography variant="h3" component="h3" sx={{ fontSize: 28 }}>
            {t.scenes.title}
          </Typography>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={1.5} sx={{ minWidth: 0 }}>
            <TextField
              select
              size="small"
              label={t.scenes.mode}
              value={mode}
              onChange={(e) => change({ mode: e.target.value })}
              sx={{ minWidth: 250 }}
            >
              {modes.map((m) => (
                <MenuItem key={m.id} value={m.id} disabled={!!m.error}>
                  {t.modeName(m.id, m.name)}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              select
              size="small"
              label={t.scenes.length}
              value={frames}
              onChange={(e) => change({ frames: Number(e.target.value) })}
              sx={{ minWidth: 130 }}
            >
              {lengths.map((n) => (
                <MenuItem key={n} value={n}>
                  {t.frames(n)}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              select
              size="small"
              label={t.scenes.addedNoise}
              value={noiseType}
              onChange={(e) => {
                const type = e.target.value as NoiseType;
                change({
                  noiseType: type,
                  noiseLevel: type === "none" ? noiseLevel : noiseRange[type].def,
                });
              }}
              sx={{ minWidth: 150 }}
            >
              <MenuItem value="none">{t.scenes.noiseNone}</MenuItem>
              <MenuItem value="gaussian">{t.scenes.noiseGaussian}</MenuItem>
              <MenuItem value="salt_pepper">{t.scenes.noiseSaltPepper}</MenuItem>
            </TextField>
            {noiseType !== "none" && (
              <Box sx={{ width: { xs: "100%", sm: 150 }, px: 1 }}>
                <Typography variant="caption" color="text.secondary" id="noise-level-label">
                  {t.scenes.level(noiseLevel)}
                </Typography>
                <Slider
                  size="small"
                  aria-labelledby="noise-level-label"
                  value={noiseLevel}
                  min={noiseRange[noiseType].min}
                  max={noiseRange[noiseType].max}
                  step={noiseRange[noiseType].step}
                  onChange={(_, v) => change({ noiseLevel: v as number })}
                  sx={{ py: 1, display: "block" }}
                />
              </Box>
            )}
          </Stack>
        </Stack>

        <Box
          sx={{
            display: "grid",
            gap: 2.5,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              lg: "repeat(3, 1fr)",
            },
          }}
        >
          {scenes === null &&
            !error &&
            [0, 1, 2].map((i) => (
              <Skeleton
                key={i}
                variant="rounded"
                sx={{ aspectRatio: "4 / 3.9", height: "auto", borderRadius: "20px" }}
              />
            ))}
          {scenes?.map((seq) => (
            <SceneCard
              key={seq.id}
              seq={seq}
              runId={latest.get(seq.id)}
              busy={starting === seq.id}
              disabled={!mode || starting !== ""}
              onAnalyze={() => analyze(seq)}
            />
          ))}
        </Box>
        {scenes?.length === 0 && <Alert severity="info">{t.scenes.empty}</Alert>}
      </Stack>
    </Stack>
  );
}

function SceneCard({
  seq,
  runId,
  busy,
  disabled,
  onAnalyze,
}: {
  seq: SequenceInfo;
  runId?: string;
  busy: boolean;
  disabled: boolean;
  onAnalyze: () => void;
}) {
  const t = useT();
  const [hover, setHover] = useState(false);
  const [tick, setTick] = useState(0);
  const title = t.sceneTitle(seq.name);
  const about = t.sceneAbout(seq);
  const poster = posterFrame(seq);

  // While the pointer is over a card its footage plays, a few frames a second.
  useEffect(() => {
    if (!hover) {
      setTick(0);
      return;
    }
    const timer = setInterval(() => setTick((v) => v + 1), 110);
    return () => clearInterval(timer);
  }, [hover]);
  const span = Math.max(1, Math.min(48, seq.frames - poster - 1));
  const frame = poster + ((tick * 2) % span);

  return (
    <Box
      component="article"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      sx={{
        display: "flex",
        flexDirection: "column",
        borderRadius: "20px",
        overflow: "hidden",
        border: 1,
        borderColor: "divider",
        bgcolor: "background.paper",
        transition: "border-color .2s, transform .2s, box-shadow .2s",
        "&:hover": {
          borderColor: "primary.main",
          transform: "translateY(-2px)",
          boxShadow: "0 18px 48px -24px rgba(249, 140, 10, 0.5)",
        },
      }}
    >
      <Box sx={{ position: "relative", aspectRatio: "4 / 3", bgcolor: "#07060a" }}>
        <Box
          component="img"
          src={previewURL(seq.id, frame)}
          alt={t.scenes.preview(seq.id)}
          sx={{
            width: "100%",
            height: "100%",
            objectFit: "cover",
            display: "block",
            // An analysed scene is dimmed so that its heat reads as light on top.
            filter: runId && !hover ? "brightness(0.62)" : "none",
            transition: "filter .3s",
          }}
        />
        {runId && (
          <Box
            component="img"
            src={runFile(runId, "heatmap.png")}
            alt=""
            sx={{
              position: "absolute",
              inset: 0,
              width: "100%",
              height: "100%",
              objectFit: "cover",
              mixBlendMode: "screen",
              filter: "blur(2px) saturate(1.2)",
              opacity: hover ? 0.3 : 1,
              transition: "opacity .3s",
              pointerEvents: "none",
            }}
          />
        )}
        <Box
          sx={{
            position: "absolute",
            inset: 0,
            background: "linear-gradient(180deg, rgba(7,6,10,0.5) 0%, transparent 28%)",
            pointerEvents: "none",
          }}
        />
        <Typography
          variant="overline"
          sx={{ position: "absolute", top: 10, left: 14, color: "rgba(255,255,255,0.86)" }}
        >
          {t.sceneSource(seq)}
        </Typography>
      </Box>
      <Stack spacing={1.5} sx={{ p: 2.25, flexGrow: 1 }}>
        <Box sx={{ flexGrow: 1 }}>
          <Typography variant="h3" component="h4" sx={{ fontSize: 23 }}>
            {title}
          </Typography>
          {about && (
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.75 }}>
              {about}
            </Typography>
          )}
        </Box>
        <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between" }}>
          <Typography variant="caption" color="text.secondary" sx={{ fontFamily: fonts.mono }}>
            {seq.width}×{seq.height} · {t.frames(seq.frames)}
          </Typography>
          <Stack direction="row" spacing={1}>
            {runId && (
              <Button
                size="small"
                color="inherit"
                startIcon={<ReplayRoundedIcon />}
                disabled={disabled}
                onClick={onAnalyze}
                aria-label={t.scenes.analyzeAgainLabel(title)}
              >
                {t.scenes.again}
              </Button>
            )}
            {runId ? (
              <Button
                size="small"
                variant="contained"
                startIcon={<PlayArrowRoundedIcon />}
                href={routeHref({ page: "run", id: runId })}
                aria-label={t.scenes.watchLabel(title)}
              >
                {t.scenes.watch}
              </Button>
            ) : (
              <Button
                size="small"
                variant="contained"
                startIcon={<PlayArrowRoundedIcon />}
                disabled={disabled}
                loading={busy}
                onClick={onAnalyze}
                aria-label={t.scenes.analyzeLabel(title)}
              >
                {t.scenes.analyze}
              </Button>
            )}
          </Stack>
        </Stack>
      </Stack>
    </Box>
  );
}
