import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Chip from "@mui/material/Chip";
import FormControl from "@mui/material/FormControl";
import Grid from "@mui/material/Grid";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import Slider from "@mui/material/Slider";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { useEffect, useMemo, useState } from "react";
import { api, type PipelineInfo, previewURL, type SequenceInfo } from "../api";
import { categoryColor } from "../palette";
import { navigate } from "../router";

type NoiseType = "none" | "gaussian" | "salt_pepper";

const noiseRange: Record<
  Exclude<NoiseType, "none">,
  { min: number; max: number; step: number; def: number }
> = {
  gaussian: { min: 1, max: 40, step: 1, def: 10 },
  salt_pepper: { min: 0.005, max: 0.1, step: 0.005, def: 0.02 },
};

export function noiseString(type: NoiseType, level: number): string {
  return type === "none" ? "none" : `${type}:${level}`;
}

export function Launcher() {
  const [pipelines, setPipelines] = useState<PipelineInfo[]>([]);
  const [sequences, setSequences] = useState<SequenceInfo[]>([]);
  const [pipeline, setPipeline] = useState("");
  const [sequence, setSequence] = useState("");
  const [noiseType, setNoiseType] = useState<NoiseType>("none");
  const [noiseLevel, setNoiseLevel] = useState(10);
  const [frames, setFrames] = useState(150);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    Promise.all([api.pipelines(), api.sequences()])
      .then(([p, s]) => {
        setPipelines(p);
        setSequences(s);
        setPipeline((cur) => cur || p.find((x) => !x.error)?.id || "");
        setSequence((cur) => cur || s[0]?.id || "");
      })
      .catch((e: Error) => setError(e.message));
  }, []);

  const pipe = pipelines.find((p) => p.id === pipeline);
  const seq = sequences.find((s) => s.id === sequence);
  const preview = useMemo(() => {
    if (!seq) return "";
    const mid = seq.has_gt
      ? Math.floor((seq.eval_start + seq.eval_end) / 2)
      : Math.floor(seq.frames / 2);
    return previewURL(seq.id, mid);
  }, [seq]);

  const start = async () => {
    setBusy(true);
    setError("");
    try {
      const { run_id } = await api.startRun({
        pipeline,
        sequence,
        noise: noiseString(noiseType, noiseLevel),
        seed: 1,
        frames,
      });
      navigate({ page: "run", id: run_id });
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <Stack spacing={3}>
      <Typography variant="h5" component="h2">
        New run
      </Typography>
      {error && <Alert severity="error">{error}</Alert>}
      <Grid container spacing={3}>
        <Grid size={{ xs: 12, md: 6 }}>
          <Card variant="outlined">
            <CardContent>
              <Stack spacing={2.5}>
                <FormControl fullWidth>
                  <InputLabel id="pipeline-label">Pipeline</InputLabel>
                  <Select
                    labelId="pipeline-label"
                    label="Pipeline"
                    value={pipeline}
                    onChange={(e) => setPipeline(e.target.value)}
                  >
                    {pipelines.map((p) => (
                      <MenuItem key={p.id} value={p.id} disabled={!!p.error}>
                        {p.id} — {p.name}
                        {p.owner ? ` (${p.owner})` : ""}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
                {pipe && (
                  <Box>
                    {pipe.description && (
                      <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                        {pipe.description}
                      </Typography>
                    )}
                    <Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: "wrap" }}>
                      {(pipe.stage_info ?? []).map((s) => (
                        <Chip
                          key={s.key}
                          size="small"
                          variant="outlined"
                          color={categoryColor[s.category] ?? "default"}
                          label={`${s.method} · ${s.category}`}
                        />
                      ))}
                    </Stack>
                  </Box>
                )}
                <FormControl fullWidth>
                  <InputLabel id="sequence-label">Sequence</InputLabel>
                  <Select
                    labelId="sequence-label"
                    label="Sequence"
                    value={sequence}
                    onChange={(e) => setSequence(e.target.value)}
                  >
                    {sequences.map((s) => (
                      <MenuItem key={s.id} value={s.id}>
                        {s.id} ({s.frames} frames{s.has_gt ? ", ground truth" : ""})
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
                <FormControl fullWidth>
                  <InputLabel id="noise-label">Synthetic noise</InputLabel>
                  <Select
                    labelId="noise-label"
                    label="Synthetic noise"
                    value={noiseType}
                    onChange={(e) => {
                      const t = e.target.value as NoiseType;
                      setNoiseType(t);
                      if (t !== "none") setNoiseLevel(noiseRange[t].def);
                    }}
                  >
                    <MenuItem value="none">None</MenuItem>
                    <MenuItem value="gaussian">Gaussian (σ)</MenuItem>
                    <MenuItem value="salt_pepper">Salt &amp; pepper (fraction)</MenuItem>
                  </Select>
                </FormControl>
                {noiseType !== "none" && (
                  <Box sx={{ px: 1 }}>
                    <Typography variant="body2" color="text.secondary" id="noise-level-label">
                      Level: {noiseLevel}
                    </Typography>
                    <Slider
                      aria-labelledby="noise-level-label"
                      value={noiseLevel}
                      min={noiseRange[noiseType].min}
                      max={noiseRange[noiseType].max}
                      step={noiseRange[noiseType].step}
                      onChange={(_, v) => setNoiseLevel(v as number)}
                    />
                  </Box>
                )}
                <TextField
                  label="Frames"
                  type="number"
                  value={frames}
                  onChange={(e) => setFrames(Math.max(1, Number(e.target.value) || 1))}
                  slotProps={{ htmlInput: { min: 1, max: 500 } }}
                  helperText="Processing starts shortly before the frames with ground truth."
                />
                <Button
                  variant="contained"
                  size="large"
                  startIcon={<PlayArrowIcon />}
                  disabled={!pipeline || !sequence || busy}
                  onClick={start}
                >
                  Start run
                </Button>
              </Stack>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          {seq && (
            <Card variant="outlined">
              <Box
                component="img"
                src={preview}
                alt={`Preview of ${seq.id}`}
                sx={{
                  width: "100%",
                  display: "block",
                  imageRendering: "pixelated",
                  bgcolor: "black",
                }}
              />
              <CardContent>
                <Typography variant="subtitle1">{seq.name}</Typography>
                <Typography variant="body2" color="text.secondary">
                  {seq.source} · {seq.category} · {seq.width}×{seq.height} · {seq.frames} frames
                  {seq.has_gt
                    ? ` · ground truth for frames ${seq.eval_start}–${seq.eval_end}`
                    : " · no ground truth"}
                </Typography>
                {seq.source === "synthetic" && seq.path && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                    {seq.path}
                  </Typography>
                )}
              </CardContent>
            </Card>
          )}
        </Grid>
      </Grid>
    </Stack>
  );
}
