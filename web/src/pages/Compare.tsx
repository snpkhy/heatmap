import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Link from "@mui/material/Link";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Typography from "@mui/material/Typography";
import { BarChart } from "@mui/x-charts/BarChart";
import { useEffect, useState } from "react";
import { api, runFile, type Summary } from "../api";
import { decibels, fps, metric, noiseLabel } from "../format";
import { type Messages, messages, useT } from "../i18n";
import { useSeriesColors } from "../palette";
import { routeHref } from "../router";
import { fonts } from "../theme";

/** Short label of an analysis for chart axes: scene, detection mode, noise. */
export function runLabel(s: Summary, t: Messages = messages.en): string {
  const n = noiseLabel(s.noise, t.noise);
  const mode = t.modeName(s.pipeline, s.pipeline_name);
  return `${t.sceneTitle(s.sequence)} · ${mode}${n === t.noise.none ? "" : ` · ${n}`}`;
}

export function Compare({ ids }: { ids: string[] }) {
  const t = useT();
  const colors = useSeriesColors();
  const [summaries, setSummaries] = useState<Summary[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .runs()
      .then((runs) => {
        const done = runs.filter((r) => r.summary);
        const chosen = ids.length ? done.filter((r) => ids.includes(r.run_id)) : done.slice(0, 8);
        setSummaries(chosen.map((r) => r.summary as Summary));
      })
      .catch((e: Error) => setError(e.message));
  }, [ids]);

  if (error) return <Alert severity="error">{error}</Alert>;
  if (!summaries) return null;
  if (summaries.length === 0) {
    return (
      <Alert severity="info">
        {t.compare.empty[0]}
        <Link href={routeHref({ page: "runs" })}>{t.compare.empty[1]}</Link>
        {t.compare.empty[2]}
      </Alert>
    );
  }
  const labels = summaries.map((s) => runLabel(s, t));
  const scored = summaries.some((s) => s.f1 !== null);
  return (
    <Stack spacing={3}>
      <Box>
        <Typography variant="h2" component="h2" sx={{ fontSize: { xs: 34, md: 42 } }}>
          {t.compare.title}
        </Typography>
        <Typography color="text.secondary" sx={{ mt: 0.5 }}>
          {t.compare.lead(ids.length === 0)}
          {t.compare.chooseOthers[0]}
          <Link href={routeHref({ page: "runs" })}>{t.compare.chooseOthers[1]}</Link>
          {t.compare.chooseOthers[2]}
        </Typography>
      </Box>
      <Box
        sx={{
          display: "grid",
          gap: 2,
          gridTemplateColumns: {
            xs: "repeat(2, 1fr)",
            md: `repeat(${Math.min(summaries.length, 4)}, 1fr)`,
          },
        }}
      >
        {summaries.map((s) => (
          <Link
            key={s.run_id}
            href={routeHref({ page: "run", id: s.run_id })}
            underline="none"
            color="inherit"
            sx={{
              display: "block",
              borderRadius: "16px",
              overflow: "hidden",
              border: 1,
              borderColor: "divider",
              bgcolor: "background.paper",
              transition: "border-color .2s",
              "&:hover": { borderColor: "primary.main" },
            }}
          >
            <Box
              component="img"
              src={runFile(s.run_id, "heatmap.png")}
              alt={t.compare.heatmapOf(runLabel(s, t))}
              loading="lazy"
              sx={{ width: "100%", display: "block", bgcolor: "#07060a" }}
            />
            <Box sx={{ p: 1.5 }}>
              <Typography
                sx={{
                  fontFamily: fonts.display,
                  fontSize: 19,
                  fontWeight: 600,
                  letterSpacing: "-0.02em",
                  lineHeight: 1.2,
                }}
              >
                {t.sceneTitle(s.sequence)}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {t.modeName(s.pipeline, s.pipeline_name)}
                {noiseLabel(s.noise, t.noise) !== t.noise.none &&
                  ` · ${t.viewer.withNoise(noiseLabel(s.noise, t.noise))}`}
              </Typography>
            </Box>
          </Link>
        ))}
      </Box>
      {scored && (
        <Paper variant="outlined" sx={{ p: 2, borderRadius: "16px" }}>
          <Typography variant="subtitle1">{t.compare.accuracy}</Typography>
          <BarChart
            height={340}
            xAxis={[{ data: labels, scaleType: "band", tickLabelStyle: { fontSize: 11 } }]}
            yAxis={[{ min: 0, max: 1 }]}
            series={[
              { data: summaries.map((s) => s.f1), label: t.metrics.f1, color: colors[0] },
              {
                data: summaries.map((s) => s.precision),
                label: t.metrics.precision,
                color: colors[1],
              },
              { data: summaries.map((s) => s.recall), label: t.metrics.recall, color: colors[2] },
            ]}
            grid={{ horizontal: true }}
          />
        </Paper>
      )}
      <TableContainer component={Paper} variant="outlined" sx={{ borderRadius: "16px" }}>
        <Table size="small" aria-label={t.compare.table}>
          <TableHead>
            <TableRow>
              <TableCell>{t.compare.scene}</TableCell>
              <TableCell>{t.compare.mode}</TableCell>
              <TableCell>{t.compare.noise}</TableCell>
              <TableCell align="right">{t.compare.precision}</TableCell>
              <TableCell align="right">{t.compare.recall}</TableCell>
              <TableCell align="right">F1</TableCell>
              <TableCell align="right">IoU</TableCell>
              <TableCell align="right">PSNR</TableCell>
              <TableCell align="right">{t.compare.speed}</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {summaries.map((s) => (
              <TableRow key={s.run_id} hover>
                <TableCell>
                  <Link href={routeHref({ page: "run", id: s.run_id })}>
                    {t.sceneTitle(s.sequence)}
                  </Link>
                </TableCell>
                <TableCell>{t.modeName(s.pipeline, s.pipeline_name)}</TableCell>
                <TableCell>{noiseLabel(s.noise, t.noise)}</TableCell>
                <TableCell align="right">{metric(s.precision)}</TableCell>
                <TableCell align="right">{metric(s.recall)}</TableCell>
                <TableCell align="right">{metric(s.f1)}</TableCell>
                <TableCell align="right">{metric(s.iou)}</TableCell>
                <TableCell align="right">{decibels(s.psnr, t.units.db)}</TableCell>
                <TableCell align="right">
                  {fps(s.fps)} {t.units.fps}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Stack>
  );
}
