import { createContext, type ReactNode, useContext, useEffect, useMemo, useState } from "react";
import type { RunStatus, SequenceInfo } from "./api";
import { sceneSource, sceneTitle } from "./format";

// The words of the interface, in English and in Ukrainian. Pages read them
// with useT(); what the server sends in English (names of detection modes,
// descriptions of the built-in scenes) is translated here by id, and anything
// unknown is shown as the server sent it.

export type Lang = "en" | "uk";

const STORAGE_KEY = "heatmap.lang";

const en = {
  lang: "en" as Lang,
  /** BCP 47 tag for dates. */
  locale: "en",
  units: { fps: "fps", ms: "ms", db: "dB" },
  frames: (n: number) => `${n} frames`,
  frame: (n: number) => `Frame ${n}`,
  status: {
    queued: "queued",
    running: "running",
    done: "done",
    failed: "failed",
    cancelled: "cancelled",
  } as Record<RunStatus, string>,
  noise: { none: "none", gaussian: "gaussian", saltPepper: "salt & pepper" },
  compass: [
    "→ right",
    "↗ up-right",
    "↑ up",
    "↖ up-left",
    "← left",
    "↙ down-left",
    "↓ down",
    "↘ down-right",
  ],
  metrics: { f1: "F1", precision: "precision", recall: "recall" },
  sceneTitle: (idOrName: string) => sceneTitle(idOrName),
  sceneAbout: (seq: SequenceInfo) => (seq.source === "synthetic" ? (seq.path ?? "") : ""),
  sceneSource: (seq: Pick<SequenceInfo, "source" | "category">) => sceneSource(seq),
  /** The name of a detection mode; `name` is what the server calls it. */
  modeName: (_id: string | undefined, name: string) => name,
  nav: {
    main: "main",
    scenes: "Scenes",
    history: "History",
    language: "language",
    toggleScheme: "toggle color scheme",
    lightTheme: "Light theme",
    darkTheme: "Dark theme",
  },
  scenes: {
    eyebrow: "Motion heatmaps from camera footage",
    headline: ["See where people ", "walk", "."],
    lead: "Pick a scene. Everyone who moves through it leaves a trail of heat, and the trails add up to a map of how the space is really used.",
    title: "Scenes",
    mode: "Detection mode",
    length: "Length",
    addedNoise: "Added noise",
    noiseNone: "None",
    noiseGaussian: "Gaussian",
    noiseSaltPepper: "Salt & pepper",
    level: (v: number) => `Level ${v}`,
    empty: "No scenes found.",
    preview: (id: string) => `Preview of ${id}`,
    analyze: "Analyze",
    again: "Again",
    watch: "Watch",
    analyzeLabel: (title: string) => `Analyze ${title}`,
    analyzeAgainLabel: (title: string) => `Analyze ${title} again`,
    watchLabel: (title: string) => `Watch ${title}`,
  },
  viewer: {
    analysis: "Analysis",
    frameRange: (a: number, b: number) => `frames ${a}–${b}`,
    withNoise: (noise: string) => `${noise} noise`,
    queue: "Waiting in queue…",
    runProgress: "run progress",
    loadingFrames: "loading frames",
    live: (done: number, total: number, speed: string) =>
      `Frame ${done} of ${total} · ${speed} fps`,
    stage: "Footage with the heat layer",
    quiet: "quiet",
    busy: "busy",
    play: "play",
    pause: "pause",
    frameSlider: "frame",
    speed: "Playback speed",
    speedLabel: "playback speed",
    view: "view",
    trails: "Trails",
    heatmap: "Heatmap",
    videoOnly: "Video only",
    palette: "palette",
    palettes: { inferno: "Inferno", turbo: "Thermal", ice: "Ice" },
    paletteLabel: (name: string) => `${name} palette`,
    intensity: "Intensity",
    intensityLabel: "intensity",
    trailLength: "Trail length",
    trailLengthLabel: "trail length",
    thermal: "Thermal view",
    outline: "Outline",
    movingNow: "Moving now",
    movingNowNote: "of the picture is in motion",
    covered: "Ground covered",
    coveredNote: "has seen movement so far",
    busiest: "Busiest moment",
    jump: "Jump there",
    planAlt: "Heatmap on the floor plan",
    planNote: "The same heat seen from above, on the floor plan",
    direction: "Main direction",
    directionNote: (n: number) => `from ${n} tracked movements`,
    accuracy: "Accuracy",
    accuracyNote: (frames: number, precision: string, recall: string) =>
      `F1 on ${frames} hand-labelled frames · precision ${precision} · recall ${recall}`,
    speedStat: "Speed",
    speedNote: (ms: string, frames: number) => `${ms} ms for ${frames} frames`,
    denoise: "Noise removal",
    noFrames:
      "This analysis was saved without its frames, so there is nothing to play — only the final heatmap. Analyses started here always keep their frames.",
    overlay: "heatmap over the scene",
    heatmapPicture: "heatmap",
    details: "details",
    how: "How it was detected",
    charts: "Charts",
    input: "input",
    mask: "motion mask",
    truth: "ground truth",
    stepByStep: (frame: number, f1: string | null, ms: string) =>
      `Frame ${frame}, step by step${f1 === null ? "" : ` · F1 ${f1}`} · ${ms} ms. Pause to inspect an exact frame.`,
    accuracyPerFrame: "Accuracy per frame",
    frameAxis: "frame",
    noTruth: "This scene has no hand-labelled frames, so accuracy cannot be measured.",
    timePerStep: "Time per step (ms per frame)",
    msPerFrame: "ms per frame",
    directions: (n: number) => `Where movement goes (${n} vectors)`,
    vectors: "vectors",
  },
  history: {
    title: "History",
    lead: "Every scene you have analysed. Tick two or more to compare them.",
    compare: (n: number) => (n > 0 ? `Compare (${n})` : "Compare"),
    empty: ["Nothing analysed yet. ", "Pick a scene", " to start."],
    table: "analyses",
    scene: "Scene",
    status: "Status",
    mode: "Detection mode",
    noise: "Noise",
    accuracy: "Accuracy (F1)",
    speed: "Speed",
    when: "When",
    remove: "Delete",
    cancel: "Cancel",
    selectLabel: (id: string) => `select ${id}`,
    openLabel: (id: string) => `open ${id}`,
    removeLabel: (id: string) => `delete ${id}`,
  },
  compare: {
    title: "Compare",
    empty: ["Nothing to compare yet. Analyse a few scenes, then pick them in the ", "history", "."],
    lead: (latest: boolean): string =>
      latest ? "The latest analyses, side by side. " : "Side by side. ",
    chooseOthers: ["Choose others in the ", "history", "."],
    heatmapOf: (label: string) => `Heatmap of ${label}`,
    accuracy: "Accuracy",
    table: "comparison",
    scene: "Scene",
    mode: "Detection mode",
    noise: "Noise",
    precision: "Precision",
    recall: "Recall",
    speed: "Speed",
  },
};

export type Messages = typeof en;

/** Ukrainian plural: 1 кадр, 2 кадри, 5 кадрів, 21 кадр… */
function plural(n: number, one: string, few: string, many: string): string {
  const d = Math.abs(n) % 100;
  const u = d % 10;
  if (d > 10 && d < 20) return many;
  if (u === 1) return one;
  if (u >= 2 && u <= 4) return few;
  return many;
}

const ukScenes: Record<string, string> = {
  plaza: "Площа",
  street: "Вулиця",
  shadow: "Тінь",
  night: "Ніч",
  pedestrians: "Пішоходи",
  peopleInShade: "Люди в тіні",
  busStation: "Автостанція",
  highway: "Шосе",
  skating: "Ковзанка",
  blizzard: "Хуртовина",
  streetCornerAtNight: "Ріг вулиці вночі",
  tramCrossroad_1fps: "Трамвайне перехрестя",
};

const ukAbout: Record<string, string> = {
  plaza: "Мощена площа, вид згори від дверей: одинадцять людей перетинають її в різних напрямках",
  street: "Дорога з розміткою: машина їде праворуч, пішохід іде по діагоналі",
  shadow: "Мощений майданчик з об'єктом, що кидає різку тінь (пікселі тіні позначені окремо)",
  night: "Темна малоконтрастна сцена: два тьмяні об'єкти рухаються назустріч",
};

const ukCategories: Record<string, string> = {
  baseline: "базові",
  shadow: "тіні",
  badWeather: "негода",
  nightVideos: "нічні",
  lowFramerate: "мало кадрів за секунду",
  cameraJitter: "тремтіння камери",
  dynamicBackground: "рухомий фон",
  thermal: "тепловізор",
  turbulence: "турбулентність",
};

const ukModes: Record<string, string> = {
  p1: "Базове віднімання фону",
  p2: "Ніч і темні сцени",
  p3: "Контури, стійкі до освітлення",
  p4: "Текстура, стійка до тіней",
  p5: "Теплова карта згори",
  p6: "Напрямки руху",
  p7: "Тіні та зміни освітлення",
  p8: "Дрібний шум і негода",
};

const lastPart = (idOrName: string) => idOrName.split("/").pop() ?? idOrName;

const uk: Messages = {
  lang: "uk",
  locale: "uk-UA",
  units: { fps: "кадр/с", ms: "мс", db: "дБ" },
  frames: (n) => `${n} ${plural(n, "кадр", "кадри", "кадрів")}`,
  frame: (n) => `Кадр ${n}`,
  status: {
    queued: "у черзі",
    running: "триває",
    done: "готово",
    failed: "помилка",
    cancelled: "скасовано",
  },
  noise: { none: "немає", gaussian: "гаусів", saltPepper: "сіль і перець" },
  compass: [
    "→ праворуч",
    "↗ вгору праворуч",
    "↑ вгору",
    "↖ вгору ліворуч",
    "← ліворуч",
    "↙ вниз ліворуч",
    "↓ вниз",
    "↘ вниз праворуч",
  ],
  metrics: { f1: "F1", precision: "влучність", recall: "повнота" },
  sceneTitle: (idOrName) => ukScenes[lastPart(idOrName)] ?? sceneTitle(idOrName),
  sceneAbout: (seq) => (seq.source === "synthetic" ? (ukAbout[seq.name] ?? seq.path ?? "") : ""),
  sceneSource: (seq) => {
    if (seq.source === "synthetic") return "Вбудована";
    if (seq.source === "frames") return "Ваше відео";
    return `CDnet · ${ukCategories[seq.category] ?? sceneTitle(seq.category).toLowerCase()}`;
  },
  modeName: (id, name) => (id && ukModes[id]) || name,
  nav: {
    main: "головне меню",
    scenes: "Сцени",
    history: "Історія",
    language: "мова",
    toggleScheme: "перемкнути тему",
    lightTheme: "Світла тема",
    darkTheme: "Темна тема",
  },
  scenes: {
    eyebrow: "Теплові карти руху з відео камер",
    headline: ["Побачте, де люди ", "ходять", "."],
    lead: "Оберіть сцену. Кожен, хто нею рухається, лишає тепловий слід, а сліди складаються в карту того, як простором користуються насправді.",
    title: "Сцени",
    mode: "Режим детекції",
    length: "Тривалість",
    addedNoise: "Доданий шум",
    noiseNone: "Немає",
    noiseGaussian: "Гаусів",
    noiseSaltPepper: "Сіль і перець",
    level: (v) => `Рівень ${v}`,
    empty: "Сцен не знайдено.",
    preview: (id) => `Прев'ю ${id}`,
    analyze: "Аналізувати",
    again: "Ще раз",
    watch: "Дивитись",
    analyzeLabel: (title) => `Аналізувати: ${title}`,
    analyzeAgainLabel: (title) => `Аналізувати ще раз: ${title}`,
    watchLabel: (title) => `Дивитись: ${title}`,
  },
  viewer: {
    analysis: "Аналіз",
    frameRange: (a, b) => `кадри ${a}–${b}`,
    withNoise: (noise) => `шум: ${noise}`,
    queue: "Чекає в черзі…",
    runProgress: "перебіг аналізу",
    loadingFrames: "завантаження кадрів",
    live: (done, total, speed) => `Кадр ${done} з ${total} · ${speed} кадр/с`,
    stage: "Відео з тепловим шаром",
    quiet: "тихо",
    busy: "людно",
    play: "відтворити",
    pause: "пауза",
    frameSlider: "кадр",
    speed: "Швидкість відтворення",
    speedLabel: "швидкість відтворення",
    view: "вигляд",
    trails: "Сліди",
    heatmap: "Теплова карта",
    videoOnly: "Лише відео",
    palette: "палітра",
    palettes: { inferno: "Інферно", turbo: "Тепловізор", ice: "Лід" },
    paletteLabel: (name) => `Палітра «${name}»`,
    intensity: "Інтенсивність",
    intensityLabel: "інтенсивність",
    trailLength: "Довжина сліду",
    trailLengthLabel: "довжина сліду",
    thermal: "Тепловий вигляд",
    outline: "Контур",
    movingNow: "Рух зараз",
    movingNowNote: "кадру зараз у русі",
    covered: "Пройдена площа",
    coveredNote: "сцени, де вже був рух",
    busiest: "Найжвавіший момент",
    jump: "Перейти",
    planAlt: "Теплова карта на плані",
    planNote: "Те саме тепло згори, на плані приміщення",
    direction: "Головний напрямок",
    directionNote: (n) =>
      `за ${n} ${plural(n, "відстеженим рухом", "відстеженими рухами", "відстеженими рухами")}`,
    accuracy: "Точність",
    accuracyNote: (frames, precision, recall) =>
      `F1 на ${frames} ${plural(frames, "розміченому кадрі", "розмічених кадрах", "розмічених кадрах")} · влучність ${precision} · повнота ${recall}`,
    speedStat: "Швидкість",
    speedNote: (ms, frames) => `${ms} мс на ${frames} ${plural(frames, "кадр", "кадри", "кадрів")}`,
    denoise: "Прибирання шуму",
    noFrames:
      "Цей аналіз збережено без кадрів, тому відтворювати нічого — є лише підсумкова теплова карта. Аналізи, запущені звідси, завжди зберігають кадри.",
    overlay: "теплова карта поверх сцени",
    heatmapPicture: "теплова карта",
    details: "деталі",
    how: "Як це знайдено",
    charts: "Графіки",
    input: "вхід",
    mask: "маска руху",
    truth: "еталон",
    stepByStep: (frame, f1, ms) =>
      `Кадр ${frame}, крок за кроком${f1 === null ? "" : ` · F1 ${f1}`} · ${ms} мс. Поставте на паузу, щоб роздивитись конкретний кадр.`,
    accuracyPerFrame: "Точність за кадрами",
    frameAxis: "кадр",
    noTruth: "Для цієї сцени немає розмічених кадрів, тому точність виміряти не можна.",
    timePerStep: "Час на крок (мс на кадр)",
    msPerFrame: "мс на кадр",
    directions: (n) => `Куди йде рух (${n} ${plural(n, "вектор", "вектори", "векторів")})`,
    vectors: "вектори",
  },
  history: {
    title: "Історія",
    lead: "Усі сцени, які ви аналізували. Позначте дві або більше, щоб порівняти.",
    compare: (n) => (n > 0 ? `Порівняти (${n})` : "Порівняти"),
    empty: ["Ще нічого не проаналізовано. ", "Оберіть сцену", ", щоб почати."],
    table: "аналізи",
    scene: "Сцена",
    status: "Стан",
    mode: "Режим детекції",
    noise: "Шум",
    accuracy: "Точність (F1)",
    speed: "Швидкість",
    when: "Коли",
    remove: "Видалити",
    cancel: "Скасувати",
    selectLabel: (id) => `обрати ${id}`,
    openLabel: (id) => `відкрити ${id}`,
    removeLabel: (id) => `видалити ${id}`,
  },
  compare: {
    title: "Порівняння",
    empty: ["Порівнювати поки нічого. Проаналізуйте кілька сцен і позначте їх в ", "історії", "."],
    lead: (latest) => (latest ? "Останні аналізи поруч. " : "Поруч. "),
    chooseOthers: ["Інші можна обрати в ", "історії", "."],
    heatmapOf: (label) => `Теплова карта: ${label}`,
    accuracy: "Точність",
    table: "порівняння",
    scene: "Сцена",
    mode: "Режим детекції",
    noise: "Шум",
    precision: "Влучність",
    recall: "Повнота",
    speed: "Швидкість",
  },
};

export const messages: Record<Lang, Messages> = { en, uk };

export const LANGUAGES: { id: Lang; label: string; name: string }[] = [
  { id: "en", label: "EN", name: "English" },
  { id: "uk", label: "УК", name: "Українська" },
];

/** The saved language, else Ukrainian for a Ukrainian browser, else English. */
export function initialLang(): Lang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === "en" || saved === "uk") return saved;
  } catch {
    // Storage can be blocked; the browser language still decides.
  }
  return typeof navigator !== "undefined" && navigator.language?.toLowerCase().startsWith("uk")
    ? "uk"
    : "en";
}

interface Language {
  lang: Lang;
  setLang: (lang: Lang) => void;
  t: Messages;
}

// Without a provider (in tests of single pages) the interface is English.
const LanguageContext = createContext<Language>({ lang: "en", setLang: () => {}, t: en });

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLang] = useState<Lang>(initialLang);

  useEffect(() => {
    document.documentElement.lang = lang;
    try {
      localStorage.setItem(STORAGE_KEY, lang);
    } catch {
      // Not remembered, but still switched.
    }
  }, [lang]);

  const value = useMemo(() => ({ lang, setLang, t: messages[lang] }), [lang]);
  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

/** The words of the current language. */
export function useT(): Messages {
  return useContext(LanguageContext).t;
}

export function useLanguage(): Language {
  return useContext(LanguageContext);
}
