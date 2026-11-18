import PauseIcon from "@mui/icons-material/Pause";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Chip from "@mui/material/Chip";
import Grid from "@mui/material/Grid";
import IconButton from "@mui/material/IconButton";
import LinearProgress from "@mui/material/LinearProgress";
import Paper from "@mui/material/Paper";
import Slider from "@mui/material/Slider";
import Stack from "@mui/material/Stack";
import Tab from "@mui/material/Tab";
import Tabs from "@mui/material/Tabs";
import Typography from "@mui/material/Typography";
import { BarChart } from "@mui/x-charts/BarChart";
import { LineChart } from "@mui/x-charts/LineChart";
import { useCallback, useEffect, useState } from "react";
import {
  api,
  type DirectionHistogram,
  eventsURL,
  type Progress,
  type Run,
  type RunDetail,
  runFile,
} from "../api";
import { compass, dateTime, decibels, fps, metric, noiseLabel, stageLabel } from "../format";
import { useSeriesColors } from "../palette";
import { statusColor } from "./RunsPage";

export function RunPage({ id }: { id: string }) {
  const [detail, setDetail] = useState<RunDetail | null>(null);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState(0);

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
      const t = setInterval(load, 1000);
      return () => clearInterval(t);
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
  return (
    <Stack spacing={3}>
      <Stack direction="row" spacing={2} sx={{ alignItems: "center", flexWrap: "wrap" }}>
        <Typography
          variant="h5"
          component="h2"
          sx={{ fontFamily: "monospace", wordBreak: "break-all" }}
        >
          {id}
        </Typography>
        <Chip color={statusColor[detail.status]} label={detail.status} />
      </Stack>
      {detail.error && <Alert severity="error">{detail.error}</Alert>}
      {active && <LiveProgress id={id} progress={progress} />}
      {run && (
        <>
          <RunHeader run={run} />
          <StatTiles run={run} />
          <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
            <Tabs value={tab} onChange={(_, v) => setTab(v)} aria-label="run views">
              <Tab label="Frames" />
              <Tab label="Heatmap" />
              <Tab label="Metrics" />
            </Tabs>
          </Box>
          {tab === 0 && <FrameViewer id={id} run={run} />}
          {tab === 1 && <HeatmapView id={id} run={run} />}
          {tab === 2 && <MetricsView run={run} />}
        </>
      )}
    </Stack>
  );
}

function LiveProgress({ id, progress }: { id: string; progress: Progress | null }) {
  const pct = progress && progress.total > 0 ? (100 * progress.done) / progress.total : 0;
  return (
    <Card variant="outlined">
      <CardContent>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          {progress
            ? `Frame ${progress.done} of ${progress.total} · ${fps(progress.fps)} fps`
            : "Waiting in queue…"}
        </Typography>
        <LinearProgress variant="determinate" value={pct} aria-label="run progress" />
        {progress && progress.done > 0 && (
          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            {["input.png", "mask.png"].map((f) => (
              <Box
                key={f}
                component="img"
                src={runFile(id, `frames/${progress.frame}/${f}`)}
                alt={`live ${f}`}
                sx={{ width: 240, maxWidth: "45%", imageRendering: "pixelated", borderRadius: 1 }}
              />
            ))}
          </Stack>
        )}
      </CardContent>
    </Card>
  );
}

function RunHeader({ run }: { run: Run }) {
  const s = run.summary;
  return (
    <Typography color="text.secondary">
      <b>{s.pipeline}</b> {s.pipeline_name} · {s.sequence} · frames {s.start}–
      {s.start + s.frames - 1} · noise {noiseLabel(s.noise)} · {dateTime(s.created_at)}
    </Typography>
  );
}

function StatTiles({ run }: { run: Run }) {
  const s = run.summary;
  const dir = directionSummary(run);
  const tiles: [string, string, string?][] = [
    ["F1", metric(s.f1), `${s.eval_frames} frames with ground truth`],
    ["Precision", metric(s.precision)],
    ["Recall", metric(s.recall)],
    ["IoU", metric(s.iou)],
    ["Speed", `${fps(s.fps)} fps`, `${s.total_ms.toFixed(0)} ms processing`],
  ];
  if (s.psnr !== null) tiles.push(["Denoising", decibels(s.psnr), `SSIM ${metric(s.ssim)}`]);
  if (dir?.dominant_deg !== undefined)
    tiles.push(["Main direction", compass(dir.dominant_deg), `${dir.vectors} vectors`]);
  return (
    <Grid container spacing={2}>
      {tiles.map(([label, value, note]) => (
        <Grid key={label} size={{ xs: 6, sm: 4, md: 12 / Math.min(tiles.length, 7) }}>
          <Paper variant="outlined" sx={{ p: 2, height: "100%" }}>
            <Typography variant="body2" color="text.secondary">
              {label}
            </Typography>
            <Typography variant="h5" component="p">
              {value}
            </Typography>
            {note && (
              <Typography variant="caption" color="text.secondary">
                {note}
              </Typography>
            )}
          </Paper>
        </Grid>
      ))}
    </Grid>
  );
}

function directionSummary(run: Run): DirectionHistogram | undefined {
  return Object.values(run.summary.extra ?? {}).find(
    (v): v is DirectionHistogram => (v as DirectionHistogram).type === "direction_histogram",
  );
}

function Picture({ src, label }: { src: string; label: string }) {
  return (
    <Card variant="outlined">
      <Box
        component="img"
        src={src}
        alt={label}
        sx={{ width: "100%", display: "block", imageRendering: "pixelated" }}
      />
      <Typography
        variant="caption"
        sx={{ display: "block", px: 1, py: 0.5 }}
        color="text.secondary"
      >
        {label}
      </Typography>
    </Card>
  );
}

function FrameViewer({ id, run }: { id: string; run: Run }) {
  const frames = run.frames;
  const firstGT = Math.max(
    0,
    frames.findIndex((f) => f.has_gt),
  );
  const [index, setIndex] = useState(firstGT);
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    if (!playing) return;
    const t = setInterval(() => setIndex((i) => (i + 1) % frames.length), 120);
    return () => clearInterval(t);
  }, [playing, frames.length]);

  if (!run.saved_frames) {
    return (
      <Alert severity="info">
        This run was started without saving frames (use -save-frames on the CLI).
      </Alert>
    );
  }
  const f = frames[index];
  if (!f) return null;
  const pics: [string, string][] = [
    [runFile(id, `frames/${f.frame}/input.png`), "input"],
    ...run.stage_keys.map((k): [string, string] => [
      runFile(id, `frames/${f.frame}/${k}.png`),
      stageLabel(k),
    ]),
    [runFile(id, `frames/${f.frame}/mask.png`), "motion mask"],
  ];
  if (f.has_gt) pics.push([runFile(id, `frames/${f.frame}/gt.png`), "ground truth"]);

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={2} sx={{ alignItems: "center" }}>
        <IconButton aria-label={playing ? "pause" : "play"} onClick={() => setPlaying((p) => !p)}>
          {playing ? <PauseIcon /> : <PlayArrowIcon />}
        </IconButton>
        <Slider
          aria-label="frame"
          value={index}
          min={0}
          max={frames.length - 1}
          onChange={(_, v) => setIndex(v as number)}
        />
        <Typography sx={{ minWidth: 220 }} color="text.secondary">
          frame {f.frame} · F1 {metric(f.f1)} · {f.time_ms.toFixed(1)} ms
        </Typography>
      </Stack>
      <Grid container spacing={2}>
        {pics.map(([src, label]) => (
          <Grid key={label} size={{ xs: 6, sm: 4, md: 3 }}>
            <Picture src={src} label={label} />
          </Grid>
        ))}
      </Grid>
    </Stack>
  );
}

function HeatmapView({ id, run }: { id: string; run: Run }) {
  const pics: [string, string][] = [
    [runFile(id, "overlay.png"), "activity heatmap over the scene"],
    [runFile(id, "heatmap.png"), "heatmap (camera view)"],
  ];
  if (run.summary.has_plan)
    pics.push([runFile(id, "plan_heatmap.png"), "heatmap on the floor plan (top-down)"]);
  return (
    <Grid container spacing={2}>
      {pics.map(([src, label]) => (
        <Grid key={label} size={{ xs: 12, md: 4 }}>
          <Picture src={src} label={label} />
        </Grid>
      ))}
    </Grid>
  );
}

function MetricsView({ run }: { run: Run }) {
  const colors = useSeriesColors();
  const frames = run.frames;
  const x = frames.map((f) => f.frame);
  const dir = directionSummary(run);
  const hasGT = frames.some((f) => f.has_gt);
  return (
    <Stack spacing={3}>
      {hasGT ? (
        <Card variant="outlined">
          <CardContent>
            <Typography variant="subtitle1">Mask quality per frame</Typography>
            <LineChart
              height={300}
              xAxis={[{ data: x, label: "frame", scaleType: "linear" }]}
              yAxis={[{ min: 0, max: 1 }]}
              series={[
                { data: frames.map((f) => f.f1), label: "F1", color: colors[0], showMark: false },
                {
                  data: frames.map((f) => f.precision),
                  label: "precision",
                  color: colors[1],
                  showMark: false,
                },
                {
                  data: frames.map((f) => f.recall),
                  label: "recall",
                  color: colors[2],
                  showMark: false,
                },
              ]}
              grid={{ horizontal: true }}
            />
          </CardContent>
        </Card>
      ) : (
        <Alert severity="info">
          This sequence has no ground truth, so there are no mask metrics.
        </Alert>
      )}
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: dir ? 6 : 12 }}>
          <Card variant="outlined">
            <CardContent>
              <Typography variant="subtitle1">Mean time per stage (ms)</Typography>
              <BarChart
                height={260}
                layout="horizontal"
                yAxis={[
                  {
                    data: run.summary.stages.map((s) => stageLabel(s.key)),
                    scaleType: "band",
                    width: 140,
                  },
                ]}
                series={[
                  {
                    data: run.summary.stages.map((s) => Number(s.mean_ms.toFixed(3))),
                    label: "ms per frame",
                    color: colors[0],
                  },
                ]}
                hideLegend
                grid={{ vertical: true }}
              />
            </CardContent>
          </Card>
        </Grid>
        {dir && (
          <Grid size={{ xs: 12, md: 6 }}>
            <Card variant="outlined">
              <CardContent>
                <Typography variant="subtitle1">
                  Motion directions ({dir.vectors} vectors)
                </Typography>
                <BarChart
                  height={260}
                  xAxis={[{ data: dir.bin_centers.map(compass), scaleType: "band" }]}
                  series={[{ data: dir.bins, label: "vectors", color: colors[0] }]}
                  hideLegend
                  grid={{ horizontal: true }}
                />
              </CardContent>
            </Card>
          </Grid>
        )}
      </Grid>
    </Stack>
  );
}
