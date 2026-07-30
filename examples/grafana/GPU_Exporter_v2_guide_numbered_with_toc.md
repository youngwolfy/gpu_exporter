# GPU Exporter v2 - руководство по архитектуре GPU и панелям Grafana

**Назначение:** техническое руководство к дашборду `GPU Exporter v2` для инженеров эксплуатации.

---

## Содержание

- [Часть I. Как устроен и выполняет работу NVIDIA GPU](#part-i)
  - [1. Общая модель: GPU как машина высокой пропускной способности](#part-i-section-1)
  - [2. CUDA grid, block, thread](#part-i-section-2)
  - [3. Warp: основная единица аппаратного планирования](#part-i-section-3)
  - [4. Streaming Multiprocessor - SM](#part-i-section-4)
  - [5. CUDA Cores и CUDA pipelines](#part-i-section-5)
  - [6. Tensor Cores](#part-i-section-6)
  - [7. Иерархия памяти GPU](#part-i-section-7)
  - [8. Как связаны основные показатели](#part-i-section-8)
  - [9. Текущие, оконные и интегральные метрики экспортера](#part-i-section-9)
- [Часть II. Переменные дашборда](#part-ii)
  - [1. Источник данных](#part-ii-section-1)
  - [2. Проект](#part-ii-section-2)
  - [3. Сервер](#part-ii-section-3)
  - [4. GPU](#part-ii-section-4)
  - [5. Порог активности](#part-ii-section-5)
- [Часть III. Панели дашборда](#part-iii)
  - [1. GPU статус](#part-iii-section-1)
  - [2. Exporter / DCGM status](#part-iii-section-2)
  - [3. Полнота сбора](#part-iii-section-3)
  - [4. Обнаружено GPU](#part-iii-section-4)
  - [5. GPU с ошибками](#part-iii-section-5)
  - [6. Возраст успешного сбора](#part-iii-section-6)
  - [7. Длительность сбора](#part-iii-section-7)
  - [8. Утилизация GPU сейчас](#part-iii-section-8)
  - [9. Память занята сейчас](#part-iii-section-9)
  - [10. Температура сейчас](#part-iii-section-10)
  - [11. Потребление сейчас](#part-iii-section-11)
  - [12. Доля power limit](#part-iii-section-12)
  - [13. Thermal / HW throttle](#part-iii-section-13)
  - [14. Утилизация GPU %](#part-iii-section-14)
  - [15. Активные окна, блоки 5 минут](#part-iii-section-15)
  - [16. Framebuffer / VRAM usage %](#part-iii-section-16)
  - [17. Копирование памяти %](#part-iii-section-17)
  - [18. DRAM activity](#part-iii-section-18)
  - [19. Мощность](#part-iii-section-19)
  - [20. Температура и headroom](#part-iii-section-20)
  - [21. SM и memory clocks](#part-iii-section-21)
  - [22. Tensor Cores](#part-iii-section-22)
  - [23. CUDA Compute / SM pipelines](#part-iii-section-23)
  - [24. VRAM capacity](#part-iii-section-24)
  - [25. BAR1 aperture - не VRAM](#part-iii-section-25)
  - [26. Ошибки GPU-сбора за период](#part-iii-section-26)
  - [27. Ошибки экспортера за период](#part-iii-section-27)
  - [28. Последний XID](#part-iii-section-28)
  - [29. Критические флаги памяти](#part-iii-section-29)
  - [30. Причины ограничения частоты](#part-iii-section-30)
  - [31. Raw throttle bitmask](#part-iii-section-31)
  - [32. Доля времени под ограничением](#part-iii-section-32)
  - [33. Аппаратные события надёжности](#part-iii-section-33)
  - [34. Флаги восстановления памяти](#part-iii-section-34)
  - [35. Полнота profiling-телеметрии](#part-iii-section-35)
  - [36. Экспорт: KPI использования GPU за выбранный период](#part-iii-section-36)
- [Часть IV. Диагностические сценарии](#part-iv)
  - [1. Compute-bound FP32 workload](#part-iv-section-1)
  - [2. Tensor-heavy workload](#part-iv-section-2)
  - [3. Memory-bound workload](#part-iv-section-3)
  - [4. Недостаточный параллелизм](#part-iv-section-4)
  - [5. Power-bound](#part-iv-section-5)
  - [6. Thermal-limited](#part-iv-section-6)
  - [7. Telemetry problem](#part-iv-section-7)
  - [8. Memory degradation](#part-iv-section-8)
- [Часть V. Что делать при No data](#part-v)
  - [1. Проверить наличие series в Grafana Explore](#part-v-section-1)
  - [2. Проверить coverage counters](#part-v-section-2)
  - [3. Проверить DCGM profiling groups](#part-v-section-3)
  - [4. Проверить pause/resume](#part-v-section-4)
  - [5. Проверить custom project label](#part-v-section-5)
  - [6. Отличать unsupported от unavailable](#part-v-section-6)
- [Часть VI. Рекомендуемые алерты](#part-vi)
  - [1. Collector](#part-vi-section-1)
  - [2. Thermal](#part-vi-section-2)
  - [3. Reliability](#part-vi-section-3)
  - [4. Capacity](#part-vi-section-4)
- [Часть VII. Практический порядок демонстрации дашборда](#part-vii)
- [Часть VIII. Корректная выгрузка и отчётность](#part-viii)
  - [1. Почему обычный график теряет короткие пики](#part-viii-section-1)
  - [2. Правильный способ выгрузки итоговых KPI](#part-viii-section-2)
  - [3. Что обязательно включать в отчёт](#part-viii-section-3)
  - [4. Когда нужна выгрузка временного ряда](#part-viii-section-4)
  - [5. Большие и регулярные отчёты](#part-viii-section-5)
  - [6. Контроль качества перед отправкой отчёта](#part-viii-section-6)
- [Часть IX. Официальные источники, использованные при проверке](#part-ix)
  - [1. Примечание для дальнейшего редактирования](#part-ix-section-1)

---


<a id="part-i"></a>
# Часть I. Как устроен и выполняет работу NVIDIA GPU

<a id="part-i-section-1"></a>
## 1. Общая модель: GPU как машина высокой пропускной способности

CPU оптимизирован прежде всего для низкой задержки выполнения небольшого количества сложных потоков. GPU оптимизирован для высокой суммарной пропускной способности: он одновременно держит тысячи потоков и выполняет одинаковые или похожие операции над большим количеством данных.

Упрощённая схема:

```text
CPU
 └─ запускает CUDA kernel
       │
       ▼
GPU
 ├─ Front-end / command processors
 ├─ много Streaming Multiprocessors (SM)
 │   ├─ warp schedulers
 │   ├─ register file
 │   ├─ shared memory / L1
 │   ├─ FP32 / FP64 / FP16 / INT pipelines
 │   ├─ Tensor Cores
 │   ├─ load/store units
 │   └─ special-function units
 ├─ общий L2 cache
 ├─ memory controllers
 ├─ VRAM: GDDR или HBM
 └─ внешние интерфейсы: PCIe, иногда NVLink
```

GPU достигает производительности не потому, что один поток работает особенно быстро, а потому, что:

- одновременно исполняется много потоков;
- один warp ждёт память, а scheduler выдаёт инструкцию другому;
- одна группа блоков работает на одних SM, другая - на других;
- специализированные pipelines параллельно обрабатывают FP32, INT, Tensor и memory operations.

---

<a id="part-i-section-2"></a>
## 2. CUDA grid, block, thread

Когда приложение запускает CUDA kernel, оно задаёт сетку - `grid`.

```text
Grid
 ├─ Block 0
 │   ├─ Thread 0
 │   ├─ Thread 1
 │   └─ ...
 ├─ Block 1
 └─ ...
```

### 2.1. Thread

CUDA thread - логический поток выполнения kernel. У него есть:

- собственный thread index;
- собственные registers;
- локальное состояние;
- возможность читать и писать память;
- собственный путь через ветвления.

### 2.2. Thread block

Thread block - группа threads, которая назначается на один SM. Threads одного block могут:

- совместно использовать shared memory;
- синхронизироваться барьерами;
- обмениваться данными быстрее, чем через VRAM.

Block не делится между несколькими SM во время выполнения. Если block назначен на SM, его ресурсы остаются на этом SM до завершения block.

### 2.3. Grid

Grid - все blocks одного запуска kernel. Blocks независимо распределяются по доступным SM.

Если blocks меньше, чем SM, часть GPU может остаться без работы. Если blocks значительно больше числа SM, завершившиеся blocks заменяются следующими из очереди.

---

<a id="part-i-section-3"></a>
## 3. Warp: основная единица аппаратного планирования

### 3.1. Определение

Внутри thread block потоки организуются в группы по **32 threads**. Такая группа называется **warp**.

Пример block из 256 threads:

```text
Warp 0: threads   0–31
Warp 1: threads  32–63
Warp 2: threads  64–95
Warp 3: threads  96–127
Warp 4: threads 128–159
Warp 5: threads 160–191
Warp 6: threads 192–223
Warp 7: threads 224–255
```

Количество warps:

```text
256 threads / 32 = 8 warps
```

Если block содержит 100 threads, аппаратно понадобится четыре warps. В последнем warp будут активны только четыре lanes, а остальные 28 lanes не выполнят полезную работу.

Поэтому размеры block часто выбирают кратными 32, хотя это не является абсолютным правилом оптимизации.

### 3.2. Lane

Каждый thread занимает одну lane внутри warp:

```text
Warp
 ├─ lane 0
 ├─ lane 1
 ├─ ...
 └─ lane 31
```

У каждой lane собственные данные и registers, но scheduler выдаёт инструкции на уровне warp.

### 3.3. SIMT

NVIDIA использует модель **SIMT - Single Instruction, Multiple Threads**.

Упрощённо одна выданная warp-инструкция означает:

```text
lane 0:  c[0]  = a[0]  + b[0]
lane 1:  c[1]  = a[1]  + b[1]
...
lane 31: c[31] = a[31] + b[31]
```

Threads остаются независимыми логически, но аппаратно исполняются группой.

### 3.4. Resident, active, eligible, issued и stalled warps

#### 3.4.1. Resident warp

Warp уже размещён на SM, для него выделены registers и ресурсы block. Resident warps участвуют в расчёте occupancy.

#### 3.4.2. Active warp

Warp относится к выполняющемуся workload и ещё не завершён. В DCGM warp может считаться активным даже тогда, когда ожидает ответ памяти.

#### 3.4.3. Eligible warp

Следующая инструкция warp готова к выдаче. Warp не ждёт данные, барьер или освобождение pipeline.

#### 3.4.4. Issued warp

Scheduler выбрал warp и направил его инструкцию в execution pipeline.

#### 3.4.5. Stalled warp

Warp временно не может продолжить. Частые причины:

- ожидание VRAM или cache miss;
- зависимость от результата предыдущей инструкции;
- barrier/synchronization;
- занятый execution pipeline;
- branch divergence;
- ожидание atomic operation.

### 3.5. Latency hiding

Доступ к VRAM может занимать сотни циклов. GPU скрывает эту задержку, переключаясь на другие готовые warps:

```text
Warp 0 → ждёт VRAM
Warp 1 → выполняет FP32
Warp 2 → выполняет Tensor MMA
Warp 3 → делает load/store
Warp 4 → ждёт barrier
```

Чем больше подходящих resident и eligible warps, тем легче scheduler удерживает pipelines занятыми.

Отсюда важность occupancy, но высокая occupancy сама по себе не гарантирует высокую производительность.

### 3.6. Branch divergence

Если threads одного warp выбирают разные ветви:

```cuda
if (value[i] > 0) {
    positive_path();
} else {
    negative_path();
}
```

аппаратно ветви могут выполняться последовательно с разными active masks:

```text
Сначала if:
lanes 0–15  активны
lanes 16–31 выключены

Затем else:
lanes 0–15  выключены
lanes 16–31 активны
```

Warp остаётся занят, но часть lanes временно не выполняет полезную работу. Это называется divergence.

### 3.7. Memory coalescing

Warp также является важной единицей доступа к памяти.

Хороший доступ:

```text
lane 0  → address 0
lane 1  → address 1
...
lane 31 → address 31
```

Последовательные и правильно выровненные адреса могут быть объединены в небольшое количество memory transactions.

Плохой доступ:

```text
lane 0  → address 0
lane 1  → address 10000
lane 2  → address 57
lane 3  → address 900000
```

требует больше транзакций и снижает эффективную пропускную способность.

---

<a id="part-i-section-4"></a>
## 4. Streaming Multiprocessor - SM

SM - основной вычислительный модуль GPU. Конкретное число блоков и pipelines зависит от архитектуры, но концептуально SM содержит:

- несколько warp schedulers;
- большой register file;
- shared memory и L1 cache;
- FP32 execution units;
- FP64 units;
- FP16 units;
- integer units;
- Tensor Cores;
- load/store units;
- special-function units;
- механизмы выдачи и отслеживания инструкций.

### 4.1. Как block попадает на SM

Для размещения block должны поместиться его ресурсы:

```text
registers per thread × threads per block
shared memory per block
threads per block
warps per block
architectural block limits
```

Если один block использует слишком много registers или shared memory, на SM одновременно помещается меньше blocks и warps. Occupancy снижается.

### 4.2. SM Active

SM Active отвечает на вопрос:

> Какую долю пространства «все SM × время» хотя бы один warp был активен?

Это не процент загрузки арифметических ядер.

Warp, ожидающий память, может считаться активным.

### 4.3. SM Occupancy

Occupancy отвечает на вопрос:

> Какая доля от теоретически возможных resident warps находилась на SM?

Пример:

```text
SM Active    = 90%
SM Occupancy = 20%
```

означает, что работа присутствует почти постоянно, но на каждом занятом SM размещено относительно мало warps.

Причины низкой occupancy:

- маленький workload или batch;
- мало blocks;
- много registers на thread;
- много shared memory на block;
- специфическая конфигурация kernel;
- последовательный decode/workload;
- крупные resource-heavy kernels.

Высокая occupancy полезна для скрытия latency, особенно в memory-bound задачах. Но compute-bound kernel может быть эффективным и с умеренной occupancy.

---

<a id="part-i-section-5"></a>
## 5. CUDA Cores и CUDA pipelines

### 5.1. Почему нет одной метрики «CUDA Core utilization»

CUDA - программная платформа и модель выполнения, а не один физический блок.

CUDA kernel может выполнять:

- FP32;
- FP64;
- FP16;
- integer operations;
- Tensor instructions;
- load/store;
- atomics;
- reductions;
- special functions;
- синхронизацию.

Поэтому невозможно корректно сложить FP32 + FP16 + INT + Tensor и получить один процент «CUDA cores».

### 5.2. Что обычно называют CUDA Cores

В практической терминологии «CUDA cores» чаще всего связывают с обычными арифметическими execution units, особенно FP32.

Для FP32 workload наиболее близкая метрика:

```text
gpu_prof_pipe_fp32_active_ratio
```

Но низкий FP32 не означает низкую общую работу GPU: задача может использовать Tensor Cores или ждать память.

---

<a id="part-i-section-6"></a>
## 6. Tensor Cores

Tensor Cores - специализированные матричные блоки. Их типичная операция:

```text
D = A × B + C
```

где A, B, C и D - матрицы или fragments матриц.

Обычные arithmetic pipelines выполняют множество скалярных операций, а Tensor pipeline выполняет специализированные matrix multiply-accumulate инструкции.

### 6.1. Mixed precision

Распространённые схемы:

```text
FP16 × FP16 → FP32 accumulation
BF16 × BF16 → FP32 accumulation
TF32 × TF32 → FP32 accumulation
INT8 × INT8 → INT32 accumulation
```

Поддерживаемые форматы зависят от поколения GPU.

### 6.2. HMMA, IMMA, DFMA

#### 6.2.1. HMMA

Half/mixed-precision matrix multiply-accumulate. Обычно связан с FP16, BF16, TF32 и большинством AI workload.

#### 6.2.2. IMMA

Integer matrix multiply-accumulate. Используется quantized inference и целочисленными Tensor operations.

#### 6.2.3. DFMA

Double-precision matrix operations. Характернее для HPC, чем для обычного LLM inference.

### 6.3. Почему Tensor Active может быть низким при 100% utilization

`GPU utilization = 100%` означает, что один или несколько kernels выполнялись почти весь период. Это не означает, что Tensor pipeline работал постоянно.

Kernel может быть занят:

- чтением весов;
- KV-cache;
- LayerNorm;
- softmax;
- indexing;
- reductions;
- memory movement;
- синхронизацией;
- небольшими матрицами.

Для LLM decode с маленьким batch часто наблюдается высокая общая utilization, но умеренная Tensor activity и низкая occupancy.


---

<a id="part-i-section-7"></a>
## 7. Иерархия памяти GPU

От самой быстрой и маленькой к более медленной и большой:

```text
Registers
   ↓
Shared memory / L1 cache
   ↓
L2 cache
   ↓
VRAM / framebuffer: GDDR или HBM
   ↓
PCIe / NVLink / host memory
```

### 7.1. Registers

Registers находятся внутри SM и принадлежат отдельным threads.

Плюсы:

- минимальная задержка;
- высокая пропускная способность.

Ограничение:

- register file конечен;
- большое число registers per thread уменьшает количество resident warps и occupancy;
- при нехватке registers компилятор может выполнять spilling в более медленную память.

### 7.2. Shared memory

Shared memory - программно управляемая on-chip память, общая для threads одного block.

Используется для:

- повторного использования данных;
- tile-based algorithms;
- обмена внутри block;
- уменьшения обращений к VRAM.

Большая shared-memory allocation на block уменьшает число blocks, одновременно размещаемых на SM.

### 7.3. L1 и L2 cache

- L1 расположен на каждом SM и связан с unified data cache.
- L2 общий для всех SM.
- Cache уменьшает число обращений к VRAM и может резко менять реальное поведение workload.

### 7.4. VRAM / framebuffer

VRAM - физическая видеопамять: GDDR или HBM.

В exporter используется термин framebuffer memory.

Метрики capacity отвечают:

- сколько всего памяти;
- сколько занято;
- сколько свободно;
- сколько зарезервировано драйвером.

Они не отвечают на вопрос о пропускной способности.

### 7.5. DRAM interface и шина памяти

VRAM подключена к GPU через memory controllers и широкую внутреннюю шину.

Грубая теоретическая пропускная способность определяется:

```text
effective memory data rate × bus width
```

Но практическая производительность зависит также от:

- паттерна адресов;
- cache hit rate;
- read/write mix;
- размера транзакций;
- coalescing;
- bank conflicts;
- числа параллельных запросов;
- текущей memory clock;
- архитектуры GPU.

`DRAM Active` показывает долю циклов, когда интерфейс device memory передавал или принимал данные. Это ближе к интенсивности работы VRAM, чем процент занятого объёма памяти.

### 7.6. Capacity и bandwidth - разные вещи

```text
VRAM used = 95%
DRAM Active = 10%
```

Память почти полностью занята, но обращаются к ней сравнительно мало.

```text
VRAM used = 20%
DRAM Active = 95%
```

Занят небольшой объём, но он читается или записывается очень интенсивно.

### 7.7. Memory Copy Engine

Copy engine/DMA перемещает данные:

- host → device;
- device → host;
- device → device;
- иногда по peer paths.

Высокий `Memory Copy %` показывает активность copy engine, но не обязательно насыщение внутренней VRAM bandwidth.

### 7.8. BAR1

BAR1 - PCIe address aperture, через который CPU или peer device отображает участки GPU memory.

Важно:

- BAR1 не является дополнительной VRAM;
- BAR1 total не обязан равняться VRAM total;
- aperture может быть больше физической VRAM;
- BAR1 особенно важен для GPUDirect RDMA и некоторых peer mappings;
- обычная модель может занимать десятки GiB VRAM при небольшом BAR1 used.

### 7.9. PCIe и NVLink

PCIe связывает GPU с host/root complex. NVLink, если присутствует, обеспечивает GPU-to-GPU или GPU-to-fabric связь.

Они не являются внутренней шиной VRAM. Поэтому:

- DRAM Active - внутренняя память GPU;
- PCIe/NVLink throughput - внешнее перемещение данных;
- Memory Copy - активность copy engine;
- VRAM used - ёмкость.

---

<a id="part-i-section-8"></a>
## 8. Как связаны основные показатели

| Показатель | Какой вопрос задаёт |
|---|---|
| GPU utilization | Выполнялся ли хотя бы один kernel в течение sample period? |
| SM Active | Какая доля всех SM была занята active warps во времени? |
| SM Occupancy | Насколько SM заполнены resident warps? |
| FP32/FP16/FP64/INT Active | Какие arithmetic pipelines реально работали? |
| Tensor Active | Как часто работал Tensor pipeline? |
| DRAM Active | Насколько занят внутренний интерфейс VRAM? |
| Memory Copy | Насколько занят copy/DMA engine? |
| VRAM Used | Сколько памяти выделено/занято? |
| Power Draw | Сколько мощности потребляет карта? |
| SM Clock | На какой частоте работают вычислительные блоки? |
| Memory Clock | На какой частоте работает видеопамять? |
| Thermal headroom | Сколько градусов осталось до operating threshold? |

### 8.1. Почему utilization может быть 100%, а SM Active - 63%

Utilization смотрит по временной оси:

> Был ли активен хотя бы один kernel?

SM Active учитывает ещё распределение по всем SM.

Один небольшой kernel может работать непрерывно и давать 100% utilization, но задействовать только часть SM.

### 8.2. Пример профиля

```text
GPU utilization = 100%
SM Active        = 63%
SM Occupancy     = 9%
Tensor Active    = 9%
FP32 Active      = 0.7%
```

Возможная интерпретация:

- GPU непрерывно выполняет kernels;
- не все SM заняты постоянно;
- на занятых SM мало resident warps;
- Tensor и FP32 pipelines загружены слабо;
- workload может быть memory-bound, latency-bound или иметь недостаточный параллелизм;
- для LLM это похоже на decode с маленьким batch.

Чтобы различить причины, нужно добавить DRAM Active, memory-copy, power draw и application throughput.

---

<a id="part-i-section-9"></a>
## 9. Текущие, оконные и интегральные метрики экспортера

### 9.1. Current

Пример:

```text
gpu_utilization_percent_current
```

Последнее валидное значение DCGM.

### 9.2. Exporter window average / max

Примеры:

```text
gpu_utilization_percent_avg
gpu_utilization_percent
gpu_power_draw_watts_avg
gpu_power_draw_watts_max
```

Это статистика завершённого внутреннего aggregation window экспортера, а не `avg_over_time` произвольного диапазона Grafana.

Особенность: `gpu_utilization_percent` исторически является window maximum, хотя у имени нет `_max`.

### 9.3. Weighted / observed counters

Примеры:

```text
gpu_utilization_weighted_seconds_total
gpu_utilization_observed_seconds_total
```

Корректная средняя:

```promql
rate(gpu_utilization_weighted_seconds_total[$__rate_interval])
/
clamp_min(
  rate(gpu_utilization_observed_seconds_total[$__rate_interval]),
  0.001
)
```

Преимущество: интервалы без валидных samples не подменяются нулевой активностью.

### 9.4. Ноль и отсутствие серии - не одно и то же

```text
series exists, value = 0
```

означает, что поле доступно и значение действительно нулевое.

```text
series is absent
```

может означать:

- поле не поддерживается GPU/DCGM;
- profiling resource временно недоступен;
- DCGM profiling paused;
- не было валидного sample;
- конфликт с profiler;
- label filter не совпал.

Для этого в дашборде добавлена панель telemetry coverage.

---

<a id="part-ii"></a>
# Часть II. Переменные дашборда

<a id="part-ii-section-1"></a>
## 1. Источник данных

`$datasource` выбирает Prometheus datasource.

<a id="part-ii-section-2"></a>
## 2. Проект

`$project` берётся из custom label `project`.

<a id="part-ii-section-3"></a>
## 3. Сервер

`$hostname` выбирает значение exporter label `hostname`.

<a id="part-ii-section-4"></a>
## 4. GPU

`$gpu` выбирает `gpu_index`.

<a id="part-ii-section-5"></a>
## 5. Порог активности

`$active_threshold` применяется к пятиминутной activity timeline.


---

<a id="part-iii"></a>
# Часть III. Панели дашборда

<a id="part-iii-section-1"></a>
## 1. GPU статус

**Тип:** table  
**Главная метрика:** `gpu_exporter_gpu_collect_success`

Столбцы:

- Имя сервера;
- Проект;
- Видеокарта;
- Статус.

`OK` означает, что последний запланированный сбор по GPU завершился без ошибок. `ERROR` означает ошибку хотя бы одной стадии сбора.

Это **статус телеметрии**, а не подтверждение, что модель или CUDA-приложение корректно работает.

Reviewed-запрос агрегирует серии по идентичности GPU и использует `min`, поэтому случайная дубликация scrape series не создаёт ложный OK.

---

<a id="part-iii-section-2"></a>
## 2. Exporter / DCGM status

**Метрика:** `gpu_exporter_up`

Показывает, смог ли exporter запросить список GPU через DCGM.

- `1` - DCGM device query успешен;
- `0` - запрос списка устройств не удался.

Это не стандартная Prometheus-метрика `up`. Если процесс exporter полностью недоступен по HTTP, сама серия может отсутствовать. Для контроля сетевой доступности exporter желательно иметь отдельный alert на Prometheus `up`.

---

<a id="part-iii-section-3"></a>
## 3. Полнота сбора

**Метрика:** `gpu_exporter_collect_success`

- `1` - последний цикл полностью успешен;
- `0` - есть partial failure по GPU, watcher group или стадии.

Exporter может быть жив и видеть GPU, но часть полей может не собираться. Поэтому эта карточка дополняет Exporter/DCGM status.

---

<a id="part-iii-section-4"></a>
## 4. Обнаружено GPU

**Метрика:** `gpu_exporter_discovered_gpus`

Суммарное количество поддерживаемых DCGM GPU на выбранных серверах.

Проверяйте расхождение с ожидаемым inventory. Если ожидалось две карты, а обнаружена одна:

- карта могла исчезнуть с PCIe;
- DCGM не видит устройство;
- драйвер не инициализировал GPU;
- фильтры выбраны неверно.

---

<a id="part-iii-section-5"></a>
## 5. GPU с ошибками

**Метрика:** `gpu_exporter_failed_gpus`

Количество GPU, у которых в последнем цикле была хотя бы одна ошибка запланированного сбора.

Значение больше нуля требует перехода к:

- GPU status table;
- GPU collection errors;
- profiling coverage;
- DCGM availability;
- журналам exporter/DCGM.

---

<a id="part-iii-section-6"></a>
## 6. Возраст успешного сбора

**Метрика:** `gpu_exporter_last_success_timestamp_seconds`

Панель показывает максимальный возраст среди выбранных серверов:

```promql
max(time() - gpu_exporter_last_success_timestamp_seconds)
```

Это важно: использование `time() - max(timestamp)` показало бы самый свежий узел и могло скрыть устаревший.

Интерпретация зависит от scrape и collection interval. Обычно alert должен срабатывать, если возраст заметно превышает несколько ожидаемых циклов сбора.

---

<a id="part-iii-section-7"></a>
## 7. Длительность сбора

**Метрика:** `gpu_exporter_collection_duration_seconds`

Показывается максимальная длительность последнего цикла среди выбранных узлов.

Рост может означать:

- задержки DCGM;
- больше watched fields;
- проблемы драйвера;
- contention;
- большое число GPU;
- временные ошибки.

Сравнивайте длительность с интервалом collection. Цикл не должен стабильно занимать большую часть интервала.

---

<a id="part-iii-section-8"></a>
## 8. Утилизация GPU сейчас

**Метрика:** `gpu_utilization_percent_current`

Последнее текущее значение общей GPU utilization.

Это процент времени, когда выполнялся один или несколько kernels. Он не равен:

- CUDA Core utilization;
- Tensor Core utilization;
- SM Occupancy;
- достигнутому проценту theoretical FLOPS.

100% может наблюдаться при маленьком непрерывном kernel, который использует только часть SM.

---

<a id="part-iii-section-9"></a>
## 9. Память занята сейчас

**Метрика:** `gpu_framebuffer_memory_used_percent`

Текущая доля занятой VRAM.

Высокое значение означает большую allocation footprint, но не обязательно memory bottleneck.

Для bottleneck смотрите:

- DRAM Active;
- memory-copy;
- Tensor/FP32;
- application throughput.

---

<a id="part-iii-section-10"></a>
## 10. Температура сейчас

**Метрика:** `gpu_temperature_celsius`

Температура кристалла GPU.

Отдельно существует `gpu_memory_temperature_celsius` - температура физических чипов VRAM/HBM/GDDR, а не процент занятой памяти.

---

<a id="part-iii-section-11"></a>
## 11. Потребление сейчас

**Метрика:** `gpu_power_draw_watts`

Текущее потребление GPU.

Сопоставляйте с:

- power limit;
- SM clock;
- utilization;
- `sw_power_cap`;
- температурой.

---

<a id="part-iii-section-12"></a>
## 12. Доля power limit

**Метрики:**

- `gpu_power_draw_watts`;
- `gpu_power_enforced_limit_watts`;
- fallback: `gpu_power_limit_watts`.

Показывает, насколько текущее потребление близко к действующему лимиту.

Значение около 100% вместе с `sw_power_cap=1` обычно означает штатный power-bound workload, а не аппаратную неисправность.

---

<a id="part-iii-section-13"></a>
## 13. Thermal / HW throttle

Сводный флаг необычных причин ограничения clocks.

Не считается критичным:

- `gpu_idle`;
- `sw_power_cap`;
- planned clock setting;
- sync boost.

Проверяются thermal/hardware/unknown reasons. Значение `ATTENTION` требует перехода к подробным clock-event панелям.

Это intentionally simplified summary. Источник истины - детальная timeline ниже.

---

<a id="part-iii-section-14"></a>
## 14. Утилизация GPU %

Показывает три разных представления нагрузки:

### 14.1. Current

`gpu_utilization_percent_current`

Последнее валидное значение.

### 14.2. Weighted average

Отношение weighted seconds к observed seconds.

Это наиболее корректная средняя для произвольного Prometheus range.

### 14.3. Window peak

`gpu_utilization_percent`

Максимум завершённого exporter window.

Полезен для коротких inference bursts.

---

<a id="part-iii-section-15"></a>
## 15. Активные окна, блоки 5 минут

Результат:

- `1` - utilization хотя бы один раз превысила `$active_threshold` в пятиминутном блоке;
- `0` - не превысила.

Полезно для визуального поиска:

- рабочих часов;
- ночного трафика;
- редких burst;
- простаивающих GPU.

Это не число пользовательских requests.

---

<a id="part-iii-section-16"></a>
## 16. Framebuffer / VRAM usage %

Метрики:

- current;
- exporter average;
- exporter maximum.

Панель показывает заполнение VRAM во времени.

Типичные наблюдения:

- стабильная высокая линия - модель постоянно резидентна в памяти;
- ступенчатый рост - дополнительные allocations/cache;
- рост без возврата после завершения workload - возможный allocator cache или memory leak;
- низкое использование при большой карте - возможна консолидация нагрузки.

---

<a id="part-iii-section-17"></a>
## 17. Копирование памяти %

Метрики:

- `gpu_memory_copy_utilization_percent`;
- `_avg`;
- `_max`.

Это активность copy engine/DMA.

Высокая линия может означать:

- host-to-device загрузку данных;
- device-to-host выгрузку;
- device-to-device copy;
- staging;
- интенсивную подготовку batches.

Не путайте с DRAM Active. Copy engine может быть активен, но внутренний интерфейс памяти не обязательно насыщен на 100%.


---

<a id="part-iii-section-18"></a>
## 18. DRAM activity

Метрики:

- current DRAM ratio;
- weighted average;
- exporter peak.

Показывает долю циклов, когда внутренний интерфейс device memory передавал или принимал данные.

### 18.1. Высокий DRAM + низкий Tensor/FP32

Вероятен memory-bandwidth или memory-latency bottleneck.

### 18.2. Низкий DRAM + низкие pipelines

Вероятны:

- недостаточный параллелизм;
- маленький batch;
- dependencies;
- synchronization;
- CPU starvation.

### 18.3. Высокий DRAM + высокая Tensor

GPU одновременно интенсивно вычисляет и передаёт данные; это хороший профиль тяжёлого матричного workload, если application throughput высокий.

---

<a id="part-iii-section-19"></a>
## 19. Мощность

Линии:

- current power;
- exporter average;
- exporter peak;
- enforced/regular power limit.

Сценарии:

```text
power ≈ limit + sw_power_cap
```

- штатный power-bound режим.

```text
utilization high + power low
```

- workload может быть memory-latency-bound, плохо параллелен или ограничен clocks.

```text
power oscillates + clocks oscillate
```

- возможны burst workload или thermal/power control loops.

---

<a id="part-iii-section-20"></a>
## 20. Температура и headroom

Линии:

- GPU core temperature;
- VRAM temperature;
- core headroom;
- VRAM headroom.

Формула headroom:

```text
maximum operating temperature - current temperature
```

### 20.1. Headroom > 15°C

Обычно хороший запас.

### 20.2. Headroom 5–15°C

Нагрузка и охлаждение близки к рабочему диапазону; наблюдать тренд.

### 20.3. Headroom 0–5°C

Высокая вероятность software thermal limiting.

### 20.4. Headroom ≤ 0°C

Порог достигнут или превышен; требуется немедленная проверка cooling/airflow.

Фиксированные пороги по абсолютной температуре менее универсальны, чем headroom, потому что operating limits различаются по моделям GPU и памяти.

---

<a id="part-iii-section-21"></a>
## 21. SM и memory clocks

Метрики:

- `gpu_sm_clock_hertz`;
- `gpu_memory_clock_hertz`.

Сопоставляйте clocks с:

- utilization;
- power draw;
- power limit;
- thermal headroom;
- decoded clock reasons.

Пример:

```text
utilization = 100%
SM clock снижается
sw_thermal = 1
```

- GPU занят, но теряет производительность из-за температуры.

```text
utilization = 100%
power ≈ limit
sw_power_cap = 1
SM clock стабилен ниже потенциального boost
```

- power-bound режим.

---

<a id="part-iii-section-22"></a>
## 22. Tensor Cores

### 22.1. Tensor weighted average

Средняя activity Tensor pipeline только по валидным samples.

### 22.2. Tensor peak

Максимум exporter window.

### 22.3. HMMA

Floating/mixed-precision Tensor operations.

### 22.4. IMMA

Integer Tensor operations.

### 22.5. DFMA

Double-precision matrix operations.

Не все GPU/DCGM combinations публикуют все подтипы. Отсутствие HMMA/IMMA/DFMA линии не означает, что общая Tensor activity равна нулю. Сначала проверяйте общую Tensor line и telemetry coverage.

---

<a id="part-iii-section-23"></a>
## 23. CUDA Compute / SM pipelines

### 23.1. SM Active

Доля пространства SM×время с active warps.

### 23.2. SM Occupancy

Доля resident warps относительно theoretical maximum.

### 23.3. FP32

Ближайшая метрика к загрузке классических FP32 CUDA arithmetic cores.

### 23.4. FP64

Double-precision pipeline. Для обычного LLM inference часто отсутствует или близок к нулю.

### 23.5. FP16

Обычный FP16 pipeline. Не включает Tensor HMMA operations.

### 23.6. INT

Integer arithmetic pipeline.

### 23.7. Пример

```text
utilization 100%
SM Active   63%
Occupancy    9%
Tensor       9%
FP32         0.7%
```

Это не противоречие. Kernel выполняется постоянно, но не заполняет все SM/warps и мало использует FP32/Tensor pipelines. Проверяйте DRAM Active, batch size, число requests, model phase и application throughput.

---

<a id="part-iii-section-24"></a>
## 24. VRAM capacity

Линии:

- used;
- free;
- total;
- reserved.

### 24.1. Used

Память, занятая allocations.

### 24.2. Free

Доступный framebuffer space.

### 24.3. Total

Физическая доступная VRAM.

### 24.4. Reserved

Память, зарезервированная драйвером/системой. Это не обязательно memory leak приложения.

Панель не показывает bandwidth.

---

<a id="part-iii-section-25"></a>
## 25. BAR1 aperture - не VRAM

Линии:

- BAR1 used;
- BAR1 free;
- BAR1 total.

BAR1 - адресное PCIe-окно. Поэтому карта с 48 GiB VRAM может иметь 64 GiB BAR1 aperture.

Не делайте вывод:

```text
BAR1 total 64 GiB → на GPU 64 GiB физической памяти
```

Физическую память показывает `gpu_memory_total_bytes`.

BAR1 важен прежде всего для GPUDirect RDMA, peer mappings и host-visible mappings.

---

<a id="part-iii-section-26"></a>
## 26. Ошибки GPU-сбора за период

**Метрика:** `gpu_exporter_gpu_collection_errors_total`

Показывает суммарный прирост per-GPU ошибок за выбранный Grafana range.

Причина может находиться в label `reason`, но summary-card агрегирует количество. Для детальной диагностики используйте Explore:

```promql
increase(
  gpu_exporter_gpu_collection_errors_total{
    project=~"$project",
    hostname=~"$hostname",
    gpu_index=~"$gpu"
  }[$__range]
)
```

---

<a id="part-iii-section-27"></a>
## 27. Ошибки экспортера за период

**Метрика:** `gpu_exporter_collection_errors_total`

Количество неполных collection attempts за выбранный диапазон.

Отличается от per-GPU errors:

- exporter-level counter отвечает за неполный общий цикл;
- GPU-level counter разбивает ошибки по GPU и reason.

---

<a id="part-iii-section-28"></a>
## 28. Последний XID

**Метрика:** `gpu_xid_last_code`

XID - код общей ошибки NVIDIA driver, записываемой в kernel log.

Важно:

- это последний код, не количество событий;
- код может оставаться после завершения инцидента;
- timestamp и подробности берутся из journal/kernel log;
- XID является отправной точкой диагностики, а не всегда однозначным диагнозом.

Часто полезные коды:

| XID | Типичный смысл |
|---:|---|
| 13 | engine exception, нередко ошибка приложения |
| 31 | MMU fault |
| 43 | GPU stopped processing |
| 45 | channel removal, иногда штатное завершение/reset |
| 48 | uncorrectable ECC |
| 63 | memory repair/page retirement/row remap |
| 64 | remap/retirement recording failure |
| 74 | NVLink error |
| 79 | GPU fallen off bus |
| 94 | contained error |
| 95 | uncontained error |

Всегда проверяйте системный журнал по `NVRM: Xid`.

---

<a id="part-iii-section-29"></a>
## 29. Критические флаги памяти

Сводная карточка объединяет:

- retired pages pending;
- row remap pending;
- row remap failure.

`ATTENTION` не объясняет причину, а только указывает на необходимость перейти к timeline флагов.

---

<a id="part-iii-section-30"></a>
## 30. Причины ограничения частоты

**Метрика:** `gpu_clock_event_active{reason=...}`

Значения:

- `0` - причина не активна;
- `1` - причина активна.

Типичные причины:

### 30.1. gpu_idle

GPU простаивает, clocks снижены. Нормально.

### 30.2. clocks_setting

Clocks ограничены заданной настройкой/application clocks.

### 30.3. sw_power_cap

Software controller удерживает потребление в power limit. При тяжёлой нагрузке часто штатно.

### 30.4. hw_slowdown

Общий hardware slowdown. Может быть связан с temperature, power brake, fast-trigger protection или переходами clocks.

### 30.5. sync_boost

GPU синхронизирован с минимальной возможной частотой группы.

### 30.6. sw_thermal

Программное thermal-регулирование удерживает GPU и memory temperature ниже maximum operating thresholds.

### 30.7. hw_thermal

Аппаратная thermal protection резко снижает core clocks из-за слишком высокой температуры. Требует повышенного внимания.

### 30.8. hw_power_brake

Внешний/платформенный power brake, например от power supply или системы управления.

### 30.9. display_clocks

Ограничение display clocks; для headless datacenter workload обычно вторично.

---

<a id="part-iii-section-31"></a>
## 31. Raw throttle bitmask

Reviewed-дашборд предпочитает decoded reasons, но исходная метрика остаётся полезной для debug:

```text
gpu_clock_throttle_reasons
```

Основные биты:

| Значение | Причина |
|---:|---|
| 0 | нет ограничений |
| 1 | GPU idle |
| 2 | application clocks setting |
| 4 | software power cap |
| 8 | hardware slowdown |
| 16 | sync boost |
| 32 | software thermal slowdown |
| 64 | hardware thermal slowdown |
| 128 | hardware power brake |
| 256 | display clocks |

Биты суммируются:

```text
36 = 4 + 32 = software power cap + software thermal
```

### 31.1. Software thermal и Hardware thermal

**Software thermal** - регулирующий механизм: clocks оптимизируются так, чтобы GPU и memory temperatures не превышали operating limits.

**Hardware thermal** - защитный механизм: температура слишком высокая, аппаратная логика снижает core clocks очень резко. Это существенно серьёзнее устойчивого software thermal.

---

<a id="part-iii-section-32"></a>
## 32. Доля времени под ограничением

**Метрика:** `gpu_clock_violation_seconds_total`

Дашборд показывает:

```promql
rate(gpu_clock_violation_seconds_total[$__rate_interval])
```

как долю времени от 0 до 1.

Примеры:

```text
0.0 = причина почти не действовала
0.5 = действовала примерно половину окна
1.0 = действовала почти постоянно
```

Причины могут перекрываться, поэтому сумма разных линий может быть больше 100%.

Power-limit violation может быть штатной характеристикой workload. Thermal, reliability или hardware reasons требуют более внимательной оценки.

---

<a id="part-iii-section-33"></a>
## 33. Аппаратные события надёжности

Показывается прирост событий за `$__rate_interval`.

### 33.1. ECC errors

Лейблы:

- correctability;
- persistence;
- location.

Correctable ошибка была исправлена ECC. Единичное событие требует наблюдения, быстрый рост - диагностики.

Uncorrectable ошибка существенно серьёзнее и должна коррелироваться с XID/recovery action.

### 33.2. PCIe replay

Повторные передачи на PCIe link level. Рост может указывать на signal-integrity, riser, slot, retimer, motherboard или power issues.

Исторический ненулевой total менее важен, чем новые события.

### 33.3. Retired pages

Legacy-механизм исключения повреждённых framebuffer pages.

### 33.4. Remapped rows

Аппаратная замена деградирующих DRAM rows запасными rows на Ampere и более новых архитектурах.

Ненулевой remap не всегда означает немедленную неисправность: механизм существует для восстановления. Важны динамика, pending и failure.

---

<a id="part-iii-section-34"></a>
## 34. Флаги восстановления памяти

### 34.1. Retired-pages pending

Есть pages, ожидающие применения retirement.

### 34.2. Row-remap pending

Row remap ожидает применения, обычно при следующем GPU reset.

### 34.3. Row-remap failure

Механизм remapping сообщил failure. Это критичный признак и основание для немедленной диагностики, drain GPU и проверки vendor/RMA procedure.

---

<a id="part-iii-section-35"></a>
## 35. Полнота profiling-телеметрии

Показывает долю времени с валидными samples для:

- utilization;
- SM Active;
- SM Occupancy;
- DRAM;
- Tensor;
- FP32.

### 35.1. Около 1.0

Поле наблюдалось почти непрерывно.

### 35.2. Между 0 и 1

Есть gaps. Средние weighted/observed остаются корректнее обычного усреднения, но данные неполны.

### 35.3. Линия полностью отсутствует

Вероятно:

- поле не поддерживается;
- не было ни одного валидного sample;
- DCGM profiling paused;
- конфликт с developer profiler;
- несовместимая profiling group;
- label filter не совпадает.


---


<a id="part-iii-section-36"></a>
## 36. Экспорт: KPI использования GPU за выбранный период

Это специальная таблица для отчётности и CSV-выгрузки. Она намеренно отделена от обычного time-series графика утилизации.

Все запросы панели имеют режим:

```text
Instant: ON
Range:   OFF
Format:  Table
```

При этом каждая формула анализирует полный выбранный диапазон через `$__range`. Поэтому ширина панели, `Max data points`, автоматический `step`, `$__interval` и разрешение экрана не меняют итоговые KPI.

### 36.1. Колонки идентификации

- **Проект** - custom Prometheus label `project`;
- **Имя сервера** - label `hostname`;
- **GPU** - `gpu_index`;
- **Видеокарта** - `gpu_name`.

### 36.2. Средняя, %

```promql
100 *
sum by (project, hostname, gpu_index, gpu_name, gpu_uuid, pci_bus_id) (
  increase(gpu_utilization_weighted_seconds_total[$__range])
)
/
clamp_min(
  sum by (project, hostname, gpu_index, gpu_name, gpu_uuid, pci_bus_id) (
    increase(gpu_utilization_observed_seconds_total[$__range])
  ),
  1
)
```

Это средняя утилизация за время, для которого у экспортера были валидные samples. Она отвечает на вопрос: **какая доля вычислительной ёмкости использовалась в среднем за выбранный период?**

Низкая средняя не обязательно означает, что GPU не использовалась. Например, одна минута при 100% и 59 минут простоя дают календарную среднюю около 1,67%.

### 36.3. Активное время, %

```promql
100 * increase(gpu_active_seconds_total[$__range])
/
clamp_min(increase(gpu_utilization_observed_seconds_total[$__range]), 1)
```

Показывает долю наблюдаемого времени, когда utilization превышала configured active threshold экспортера.

### 36.4. Активные часы

```promql
increase(gpu_active_seconds_total[$__range]) / 3600
```

Сколько часов GPU находилась выше порога активности, независимо от того, была загрузка 10% или 100%.

### 36.5. GPU-часы (100%)

```promql
increase(gpu_utilization_weighted_seconds_total[$__range]) / 3600
```

Эквивалент работы при полной загрузке. Например:

```text
10 часов со средней утилизацией 50%
≈ 5 GPU-часов при 100%
```

Это один из наиболее удобных показателей для capacity planning и внутреннего распределения стоимости.

### 36.6. Peak, %

```promql
max_over_time(gpu_utilization_percent[$__range])
```

Максимальный сохранённый оконный пик за период. Он показывает, достигала ли карта высокой загрузки хотя бы кратковременно.

### 36.7. P95 оконных пиков, %

```promql
quantile_over_time(0.95, gpu_utilization_percent[$__range])
```

95-й перцентиль **оконных максимумов экспортера**, а не сырых momentary samples. Этот показатель устойчивее единичного peak, но его нужно подписывать именно как P95 оконных пиков.

### 36.8. Окна активности

```promql
increase(gpu_activity_windows_total[$__range])
```

Количество переходов в активные периоды, определённых логикой экспортера. Это оценка отдельных всплесков/сессий работы GPU, а не число запросов приложения.

### 36.9. Как читать строку таблицы

Используйте показатели вместе:

- **Средняя** - долгосрочная эффективность использования;
- **Активное время** - сколько времени GPU действительно была занята;
- **Активные часы** - абсолютная длительность активной работы;
- **GPU-часы** - интегральный объём выполненной GPU-работы;
- **Peak** - были ли короткие высокие всплески;
- **P95** - типичный верхний уровень без единичных экстремумов;
- **Окна активности** - насколько нагрузка фрагментирована.

Перед интерпретацией отчёта обязательно проверьте панель **«Полнота profiling-телеметрии»** и collector health: корректная формула не может восстановить samples, которых не было в Prometheus.

---

<a id="part-iv"></a>
# Часть IV. Диагностические сценарии

<a id="part-iv-section-1"></a>
## 1. Compute-bound FP32 workload

```text
GPU utilization  высокий
SM Active        высокий
FP32 Active      высокий
DRAM Active      умеренный
Power            близко к limit
```

Вывод: обычные arithmetic pipelines хорошо загружены.

<a id="part-iv-section-2"></a>
## 2. Tensor-heavy workload

```text
SM Active        высокий
Tensor Active    высокий
HMMA/IMMA        высокий
FP32             низкий
DRAM             умеренный или высокий
```

Низкий FP32 здесь нормален.

<a id="part-iv-section-3"></a>
## 3. Memory-bound workload

```text
GPU utilization  высокий
SM Active        высокий
DRAM Active      очень высокий
Tensor/FP32      низкие или умеренные
```

Kernel выполняется, но performance ограничен памятью.

<a id="part-iv-section-4"></a>
## 4. Недостаточный параллелизм

```text
GPU utilization  может быть 100%
SM Active        средний
Occupancy        низкий
DRAM             низкий/средний
Tensor/FP32      низкие
Power            ниже limit
```

Возможные причины:

- маленький batch;
- мало concurrent requests;
- мало blocks;
- маленькие матрицы;
- resource-heavy kernel;
- CPU synchronization;
- autoregressive decode.

<a id="part-iv-section-5"></a>
## 5. Power-bound

```text
Power draw       ≈ power limit
sw_power_cap     активно
SM clock         стабилен, но ниже потенциального boost
Temperature      в норме
```

Обычно штатный режим.

<a id="part-iv-section-6"></a>
## 6. Thermal-limited

```text
headroom          близок к 0
sw_thermal        активно
thermal violation доля растёт
SM clock          снижается
```

Проверить airflow, fans, inlet temperature и плотность карт.

`hw_thermal=1` - более серьёзный уровень защиты.

<a id="part-iv-section-7"></a>
## 7. Telemetry problem

```text
GPU работает
Collect success  = 0
coverage         низкий
часть lines      No data
```

Это не обязательно отсутствие активности. Сначала устраните проблему сбора.

<a id="part-iv-section-8"></a>
## 8. Memory degradation

```text
new ECC          растут
remapped rows    растут
pending          = 1
XID              48/63/64/94/95
```

Нужны drain, logs, DCGM diagnostics и vendor procedure.

---

<a id="part-v"></a>
# Часть V. Что делать при No data

<a id="part-v-section-1"></a>
## 1. Проверить наличие series в Grafana Explore

```promql
count by (__name__) (
  {
    __name__=~"gpu_prof_.*",
    project=~"$project",
    hostname=~"$hostname",
    gpu_index=~"$gpu"
  }
)
```

<a id="part-v-section-2"></a>
## 2. Проверить coverage counters

```promql
rate(
  gpu_tensor_active_observed_seconds_total{
    project=~"$project",
    hostname=~"$hostname",
    gpu_index=~"$gpu"
  }[$__rate_interval]
)
```

<a id="part-v-section-3"></a>
## 3. Проверить DCGM profiling groups

На сервере:

```bash
dcgmi profile -l -i 0
```

Команда показывает поддерживаемые profiling fields и группы совместимости.

Некоторые counters аппаратно нельзя собирать одновременно. DCGM multiplexes группы; слишком высокая частота опроса может приводить к нулям или gaps.

<a id="part-v-section-4"></a>
## 4. Проверить pause/resume

Developer profilers могут требовать profiling resources. Если DCGM profiling был приостановлен:

```bash
dcgmi profile --resume
```

Пауза действует на host engine целиком.

<a id="part-v-section-5"></a>
## 5. Проверить custom project label

```promql
count by (project, hostname) (
  gpu_exporter_gpu_collect_success
)
```

Если `project` отсутствует, панели с фильтром проекта не покажут серии.

<a id="part-v-section-6"></a>
## 6. Отличать unsupported от unavailable

Используйте:

```promql
gpu_dcgm_field_supported
gpu_dcgm_field_available
gpu_dcgm_field_last_success_timestamp_seconds
```

- supported=0 - поле не поддерживается;
- supported=1, available=0 - поддерживается, но последний read не дал валидного значения;
- available=1, value=0 - валидный настоящий ноль.

---

<a id="part-vi"></a>
# Часть VI. Рекомендуемые алерты

Пороговые значения следует адаптировать к scrape interval, моделям GPU и внутренним SLA.

<a id="part-vi-section-1"></a>
## 1. Collector

- `gpu_exporter_up == 0`;
- `gpu_exporter_collect_success == 0`;
- `gpu_exporter_failed_gpus > 0`;
- last success age превышает несколько collection intervals;
- прирост collection errors > 0.

<a id="part-vi-section-2"></a>
## 2. Thermal

- core или VRAM headroom < 5°C - warning;
- headroom ≤ 0°C - critical;
- устойчивый `sw_thermal=1` - warning/performance degradation;
- любой устойчивый `hw_thermal=1` - critical;
- `hw_power_brake=1` - critical/platform investigation.

<a id="part-vi-section-3"></a>
## 3. Reliability

- новый uncorrectable ECC > 0 - critical;
- новый PCIe replay burst - warning/critical по baseline;
- row-remap pending - warning и план reset;
- row-remap failure - critical;
- XID 48/64/74/79/95 - critical;
- coverage ниже внутреннего допустимого уровня - telemetry warning.

<a id="part-vi-section-4"></a>
## 4. Capacity

- VRAM used > 90–95% - warning только при риске OOM; высокая резидентность модели сама по себе может быть нормой.

---

<a id="part-vii"></a>
# Часть VII. Практический порядок демонстрации дашборда

1. Начните с inventory table: какие серверы, проекты и GPU находятся в scope.
2. Проверьте collector health: exporter/DCGM, completeness, failed GPUs, age и duration.
3. Посмотрите current cards: utilization, VRAM, temperature, power и thermal/hardware throttle.
4. Перейдите к utilization timeline и activity windows.
5. Оцените memory subsystem: framebuffer %, copy engine и DRAM activity.
6. Сопоставьте power, temperatures, headroom и clocks.
7. Посмотрите Tensor и CUDA/SM pipelines:
   - какая арифметика используется;
   - сколько SM занято;
   - хватает ли occupancy;
   - не является ли workload memory-bound.
8. Проверьте VRAM capacity и отдельно BAR1.
9. Проверьте exporter errors и XID.
10. Завершите clock reasons, violation time, hardware events, memory flags и telemetry coverage.
11. В конце откройте экспортную KPI-таблицу, объясните разницу между average, active time, GPU-hours, peak и P95, затем покажите корректную CSV-выгрузку.

---


<a id="part-viii"></a>
# Часть VIII. Корректная выгрузка и отчётность

<a id="part-viii-section-1"></a>
## 1. Почему обычный график теряет короткие пики

Для range query Grafana выбирает шаг (`step`) исходя из выбранного временного диапазона, ширины панели и `Max data points`.

Упрощённо:

```text
step ≈ time range / Max data points
```

При увеличении диапазона шаг растёт. В результате возникают два разных эффекта:

1. Формулы с `rate(...[$__rate_interval])` начинают усреднять данные по более длинному окну.
2. Gauge-серии могут быть вычислены только в редких точках range query, поэтому короткое промежуточное значение может вообще не попасть в возвращённый ряд.

CSV из обычной time-series панели содержит уже рассчитанный и прореженный результат запроса, а не все исходные samples Prometheus. После выгрузки восстановить пропущенные точки невозможно.

<a id="part-viii-section-2"></a>
## 2. Правильный способ выгрузки итоговых KPI

Используйте панель **«Экспорт: KPI использования GPU за выбранный период»** в самом конце дашборда.

Порядок:

1. Выберите точный диапазон времени в Grafana.
2. Проверьте timezone дашборда и границы периода.
3. Выберите `project`, `hostname` и `GPU`.
4. Дождитесь завершения всех запросов таблицы.
5. Откройте меню панели.
6. Выберите **Inspect → Data**.
7. Включите выгрузку transformed/formatted data, если ваша версия Grafana предлагает такой переключатель: нужны уже объединённые и переименованные колонки таблицы.
8. Нажмите **Download CSV**.

Эта панель использует Instant-запросы и анализирует весь `$__range`, поэтому итоговые показатели не зависят от количества пикселей и Max data points.

<a id="part-viii-section-3"></a>
## 3. Что обязательно включать в отчёт

Не заменяйте среднюю одним максимумом и не интерпретируйте среднюю отдельно. Минимальный набор:

| Показатель | На какой вопрос отвечает |
|---|---|
| Средняя, % | Насколько GPU использовалась в среднем за период? |
| Активное время, % | Какую долю времени она была выше active threshold? |
| Активные часы | Сколько часов реально была активной? |
| GPU-часы (100%) | Каков интегральный объём работы? |
| Peak, % | Достигала ли высокой загрузки хотя бы кратковременно? |
| P95 оконных пиков, % | Каков устойчивый верхний уровень без единичного экстремума? |
| Окна активности | Насколько нагрузка была фрагментирована? |

Пример:

```text
Средняя               2.1%
Активное время        3.0%
Активные часы         21.6 h
GPU-часы (100%)       15.1 h
Peak                  100%
P95 оконных пиков     82%
Окна активности       940
```

Такой профиль означает не «GPU не используется», а частые короткие интенсивные сессии на фоне длительного простоя.

<a id="part-viii-section-4"></a>
## 4. Когда нужна выгрузка временного ряда

Если нужен CSV с timestamp, заранее определите фиксированный bucket, например 1m, 5m или 15m. Экспортируйте отдельно среднее и пик каждого bucket.

Для пятиминутного peak:

```promql
max_over_time(
  gpu_utilization_percent{
    project=~"$project",
    hostname=~"$hostname",
    gpu_index=~"$gpu"
  }[5m]
)
```

Для пятиминутной средней:

```promql
100 *
increase(
  gpu_utilization_weighted_seconds_total{
    project=~"$project",
    hostname=~"$hostname",
    gpu_index=~"$gpu"
  }[5m]
)
/
clamp_min(
  increase(
    gpu_utilization_observed_seconds_total{
      project=~"$project",
      hostname=~"$hostname",
      gpu_index=~"$gpu"
    }[5m]
  ),
  0.001
)
```

Для такой панели задайте:

```text
Min interval / Min step = размер bucket
Max data points          >= число buckets в диапазоне
```

Примеры разумного разрешения:

| Диапазон | Bucket | Приблизительное число строк на серию |
|---|---:|---:|
| 24 часа | 1 минута | 1 440 |
| 7 дней | 1 минута | 10 080 |
| 30 дней | 5 минут | 8 640 |
| 90 дней | 15 минут | 8 640 |
| 1 год | 1 час | 8 760 |

Перед выгрузкой откройте **Inspect → Query** и проверьте фактический `step`. `Min step` является нижней границей, но Grafana всё равно увеличит шаг, если `Max data points` слишком мал.

<a id="part-viii-section-5"></a>
## 5. Большие и регулярные отчёты

Для регулярной отчётности лучше использовать один из двух вариантов:

1. **Prometheus recording rules** с заранее рассчитанными рядами `avg_1m`, `max_1m`, `active_fraction_1m`.
2. **Prometheus HTTP API `query_range`** с явно заданным `step`.

Это делает разрешение независимым от Grafana и снижает стоимость повторяющихся тяжёлых запросов.

<a id="part-viii-section-6"></a>
## 6. Контроль качества перед отправкой отчёта

Перед публикацией CSV:

- убедитесь, что выбран правильный timezone;
- сохраните вместе с отчётом начало и конец периода;
- зафиксируйте active threshold экспортера;
- проверьте collector errors и last-success age;
- проверьте observation/profiling coverage;
- убедитесь, что `project` присутствует у всех series;
- подпишите P95 как **P95 оконных пиков**, а не как P95 сырых samples;
- не суммируйте проценты разных GPU без явного определения методики агрегации.

---

<a id="part-ix"></a>
# Часть IX. Официальные источники, использованные при проверке

- NVIDIA CUDA Programming Guide - Programming Model, Warps and SIMT, memory hierarchy.
- NVIDIA DCGM Documentation - Profiling metrics, SM Active, SM Occupancy, Tensor Activity, DRAM Activity, multiplexing.
- NVIDIA DCGM Field Identifiers - profiling, ECC, retired pages, row remapping, PCIe replay.
- NVIDIA NVML API Reference - GPU utilization and clock event reasons.
- NVIDIA XID Errors Documentation - XID interpretation and kernel-log workflow.
- NVIDIA GPU Memory Error Management - row remapping, pending/failure flags and recovery.
- NVIDIA GPUDirect RDMA Documentation - BAR1 aperture and BAR-space usage.

---

<a id="part-ix-section-1"></a>
## 1. Примечание для дальнейшего редактирования

Рекомендуется дополнить этот файл:

- вашими фактическими scrape и collection intervals;
- моделями GPU и их expected operating ranges;
- ссылками на internal runbook;
- командами доступа к BMC;
- правилами drain/reset/reboot;
- ответственными командами;
- согласованными warning/critical thresholds;
- примерами типичных workload вашей организации.
