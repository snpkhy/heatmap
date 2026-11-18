import Alert from "@mui/material/Alert";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
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
import { api, type Summary } from "../api";
import { decibels, fps, metric, noiseLabel } from "../format";
import { useSeriesColors } from "../palette";
import { routeHref } from "../router";

/** Short label of a run for chart axes: pipeline, sequence name, noise. */
export function runLabel(s: Summary): string {
  const seq = s.sequence.split("/").pop();
  const n = noiseLabel(s.noise);
  return `${s.pipeline} · ${seq}${n === "none" ? "" : ` · ${n}`}`;
}

export function Compare({ ids }: { ids: string[] }) {
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
        Nothing to compare yet. Finish a few runs, then pick them on the{" "}
        <Link href={routeHref({ page: "runs" })}>runs page</Link>.
      </Alert>
    );
  }
  const labels = summaries.map(runLabel);
  const val = (v: number | null) => v;
  return (
    <Stack spacing={3}>
      <Typography variant="h5" component="h2">
        Compare runs
      </Typography>
      {ids.length === 0 && (
        <Typography color="text.secondary">
          Showing the latest finished runs. Select specific runs on the{" "}
          <Link href={routeHref({ page: "runs" })}>runs page</Link>.
        </Typography>
      )}
      <Card variant="outlined">
        <CardContent>
          <Typography variant="subtitle1">Mask quality</Typography>
          <BarChart
            height={340}
            xAxis={[{ data: labels, scaleType: "band", tickLabelStyle: { fontSize: 11 } }]}
            yAxis={[{ min: 0, max: 1 }]}
            series={[
              { data: summaries.map((s) => val(s.f1)), label: "F1", color: colors[0] },
              {
                data: summaries.map((s) => val(s.precision)),
                label: "precision",
                color: colors[1],
              },
              { data: summaries.map((s) => val(s.recall)), label: "recall", color: colors[2] },
            ]}
            grid={{ horizontal: true }}
          />
        </CardContent>
      </Card>
      <TableContainer component={Paper} variant="outlined">
        <Table size="small" aria-label="comparison">
          <TableHead>
            <TableRow>
              <TableCell>Run</TableCell>
              <TableCell>Pipeline</TableCell>
              <TableCell>Sequence</TableCell>
              <TableCell>Noise</TableCell>
              <TableCell align="right">Precision</TableCell>
              <TableCell align="right">Recall</TableCell>
              <TableCell align="right">F1</TableCell>
              <TableCell align="right">IoU</TableCell>
              <TableCell align="right">PSNR</TableCell>
              <TableCell align="right">FPS</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {summaries.map((s) => (
              <TableRow key={s.run_id} hover>
                <TableCell sx={{ fontFamily: "monospace", fontSize: 12 }}>
                  <Link href={routeHref({ page: "run", id: s.run_id })}>{s.run_id}</Link>
                </TableCell>
                <TableCell>
                  {s.pipeline} — {s.pipeline_name}
                </TableCell>
                <TableCell>{s.sequence}</TableCell>
                <TableCell>{noiseLabel(s.noise)}</TableCell>
                <TableCell align="right">{metric(s.precision)}</TableCell>
                <TableCell align="right">{metric(s.recall)}</TableCell>
                <TableCell align="right">{metric(s.f1)}</TableCell>
                <TableCell align="right">{metric(s.iou)}</TableCell>
                <TableCell align="right">{decibels(s.psnr)}</TableCell>
                <TableCell align="right">{fps(s.fps)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Stack>
  );
}
