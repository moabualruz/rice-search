import { Injectable, Logger } from "@nestjs/common";
import { StoreVersioningService } from "./store-versioning.service";

export interface ABStoreConfig {
  name: string;
  version: string;
}

export interface ABTestConfig {
  name: string;
  description: string;
  storeA: ABStoreConfig;
  storeB: ABStoreConfig;
  trafficSplit: number;  // 0-1, fraction to version B
  metrics: string[];
  startTime: Date;
  endTime?: Date;
  status: "active" | "paused" | "completed";
}

export interface ABMetrics {
  queryCount: number;
  avgLatencyMs: number;
  avgResultCount: number;
  avgTopScore: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
}

export interface ABTestResults {
  testName: string;
  status: "active" | "paused" | "completed";
  duration: {
    startTime: Date;
    endTime?: Date;
    durationMs: number;
  };
  variantA: {
    store: ABStoreConfig;
    queryCount: number;
    metrics: ABMetrics;
  };
  variantB: {
    store: ABStoreConfig;
    queryCount: number;
    metrics: ABMetrics;
  };
  comparison: {
    latencyDiff: number;      // B - A (negative = B faster)
    resultCountDiff: number;  // B - A
    topScoreDiff: number;     // B - A
    significanceLevel: number; // p-value approximation
  };
}

interface ABQueryRecord {
  timestamp: Date;
  variant: "A" | "B";
  latencyMs: number;
  resultCount: number;
  topScore: number;
  query: string;
}

@Injectable()
export class ABEvaluationService {
  private readonly logger = new Logger(ABEvaluationService.name);
  private readonly activeTests: Map<string, ABTestConfig> = new Map();
  private readonly testRecords: Map<string, ABQueryRecord[]> = new Map();

  // Ring buffer size for records per test
  private readonly maxRecordsPerTest = 10000;

  constructor(private storeVersioning: StoreVersioningService) {
    this.logger.log("ABEvaluationService initialized");
  }

  /**
   * Create and start an A/B test
   */
  createTest(config: Omit<ABTestConfig, "startTime" | "status">): ABTestConfig {
    // Validate stores and versions exist
    this.validateStoreVersion(config.storeA);
    this.validateStoreVersion(config.storeB);

    const testConfig: ABTestConfig = {
      ...config,
      startTime: new Date(),
      status: "active",
    };

    this.activeTests.set(config.name, testConfig);
    this.testRecords.set(config.name, []);

    this.logger.log(
      `Created A/B test: ${config.name} (${config.storeA.name}@${config.storeA.version} vs ` +
      `${config.storeB.name}@${config.storeB.version}, split=${config.trafficSplit})`
    );

    return testConfig;
  }

  /**
   * Validate that a store and version exist
   */
  private validateStoreVersion(config: ABStoreConfig): void {
    const version = this.storeVersioning.getVersion(config.name, config.version);
    if (!version) {
      throw new Error(`Store ${config.name} version ${config.version} not found`);
    }
    if (version.status !== "active" && version.status !== "ready") {
      throw new Error(
        `Store ${config.name} version ${config.version} is not ready (status: ${version.status})`
      );
    }
  }

  /**
   * Get test configuration
   */
  getTest(testName: string): ABTestConfig | undefined {
    return this.activeTests.get(testName);
  }

  /**
   * Assign a query to a variant based on traffic split
   */
  assignVariant(testName: string): { variant: "A" | "B"; store: ABStoreConfig } {
    const test = this.activeTests.get(testName);
    if (!test) {
      throw new Error(`A/B test ${testName} not found`);
    }

    if (test.status !== "active") {
      throw new Error(`A/B test ${testName} is not active`);
    }

    // Random assignment based on traffic split
    const variant = Math.random() < test.trafficSplit ? "B" : "A";
    const store = variant === "A" ? test.storeA : test.storeB;

    return { variant, store };
  }

  /**
   * Record a query result for analysis
   */
  recordQuery(
    testName: string,
    variant: "A" | "B",
    latencyMs: number,
    resultCount: number,
    topScore: number,
    query: string,
  ): void {
    const records = this.testRecords.get(testName);
    if (!records) return;

    const record: ABQueryRecord = {
      timestamp: new Date(),
      variant,
      latencyMs,
      resultCount,
      topScore,
      query,
    };

    // Ring buffer - remove oldest if at capacity
    if (records.length >= this.maxRecordsPerTest) {
      records.shift();
    }
    records.push(record);
  }

  /**
   * Get test results with statistical analysis
   */
  getResults(testName: string): ABTestResults {
    const test = this.activeTests.get(testName);
    if (!test) {
      throw new Error(`A/B test ${testName} not found`);
    }

    const records = this.testRecords.get(testName) ?? [];
    const recordsA = records.filter((r) => r.variant === "A");
    const recordsB = records.filter((r) => r.variant === "B");

    const metricsA = this.computeMetrics(recordsA);
    const metricsB = this.computeMetrics(recordsB);

    const now = new Date();
    const durationMs = (test.endTime ?? now).getTime() - test.startTime.getTime();

    return {
      testName,
      status: test.status,
      duration: {
        startTime: test.startTime,
        endTime: test.endTime,
        durationMs,
      },
      variantA: {
        store: test.storeA,
        queryCount: recordsA.length,
        metrics: metricsA,
      },
      variantB: {
        store: test.storeB,
        queryCount: recordsB.length,
        metrics: metricsB,
      },
      comparison: {
        latencyDiff: metricsB.avgLatencyMs - metricsA.avgLatencyMs,
        resultCountDiff: metricsB.avgResultCount - metricsA.avgResultCount,
        topScoreDiff: metricsB.avgTopScore - metricsA.avgTopScore,
        significanceLevel: this.computeSignificance(recordsA, recordsB),
      },
    };
  }

  /**
   * Compute metrics for a set of query records
   */
  private computeMetrics(records: ABQueryRecord[]): ABMetrics {
    if (records.length === 0) {
      return {
        queryCount: 0,
        avgLatencyMs: 0,
        avgResultCount: 0,
        avgTopScore: 0,
        p50LatencyMs: 0,
        p95LatencyMs: 0,
      };
    }

    const latencies = records.map((r) => r.latencyMs).sort((a, b) => a - b);
    const avgLatencyMs = latencies.reduce((a, b) => a + b, 0) / latencies.length;
    const avgResultCount = records.reduce((a, r) => a + r.resultCount, 0) / records.length;
    const avgTopScore = records.reduce((a, r) => a + r.topScore, 0) / records.length;

    const p50Idx = Math.floor(latencies.length * 0.5);
    const p95Idx = Math.floor(latencies.length * 0.95);

    return {
      queryCount: records.length,
      avgLatencyMs,
      avgResultCount,
      avgTopScore,
      p50LatencyMs: latencies[p50Idx] ?? 0,
      p95LatencyMs: latencies[p95Idx] ?? 0,
    };
  }

  /**
   * Compute approximate significance level using Welch's t-test
   * Returns p-value approximation (lower = more significant)
   */
  private computeSignificance(recordsA: ABQueryRecord[], recordsB: ABQueryRecord[]): number {
    if (recordsA.length < 30 || recordsB.length < 30) {
      // Not enough data for significance
      return 1.0;
    }

    const latenciesA = recordsA.map((r) => r.latencyMs);
    const latenciesB = recordsB.map((r) => r.latencyMs);

    const meanA = latenciesA.reduce((a, b) => a + b, 0) / latenciesA.length;
    const meanB = latenciesB.reduce((a, b) => a + b, 0) / latenciesB.length;

    const varA = latenciesA.reduce((a, x) => a + Math.pow(x - meanA, 2), 0) / (latenciesA.length - 1);
    const varB = latenciesB.reduce((a, x) => a + Math.pow(x - meanB, 2), 0) / (latenciesB.length - 1);

    const seA = varA / latenciesA.length;
    const seB = varB / latenciesB.length;
    const se = Math.sqrt(seA + seB);

    if (se === 0) return 1.0;

    const t = Math.abs(meanA - meanB) / se;

    // Approximate p-value from t-statistic (rough approximation)
    // For large samples, t-distribution approaches normal
    const p = Math.exp(-0.5 * t * t) * 2;

    return Math.min(1.0, p);
  }

  /**
   * Pause a test
   */
  pauseTest(testName: string): void {
    const test = this.activeTests.get(testName);
    if (!test) {
      throw new Error(`A/B test ${testName} not found`);
    }

    test.status = "paused";
    this.logger.log(`Paused A/B test: ${testName}`);
  }

  /**
   * Resume a paused test
   */
  resumeTest(testName: string): void {
    const test = this.activeTests.get(testName);
    if (!test) {
      throw new Error(`A/B test ${testName} not found`);
    }

    if (test.status !== "paused") {
      throw new Error(`A/B test ${testName} is not paused`);
    }

    test.status = "active";
    this.logger.log(`Resumed A/B test: ${testName}`);
  }

  /**
   * Complete a test
   */
  completeTest(testName: string): ABTestResults {
    const test = this.activeTests.get(testName);
    if (!test) {
      throw new Error(`A/B test ${testName} not found`);
    }

    test.status = "completed";
    test.endTime = new Date();

    this.logger.log(`Completed A/B test: ${testName}`);
    return this.getResults(testName);
  }

  /**
   * Delete a test and its records
   */
  deleteTest(testName: string): void {
    this.activeTests.delete(testName);
    this.testRecords.delete(testName);
    this.logger.log(`Deleted A/B test: ${testName}`);
  }

  /**
   * List all tests
   */
  listTests(): ABTestConfig[] {
    return Array.from(this.activeTests.values());
  }
}
