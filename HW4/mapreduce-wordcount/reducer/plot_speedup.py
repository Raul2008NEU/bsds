import matplotlib.pyplot as plt

# Your data here
sequential_time = 0.48  # Replace with your actual value
parallel_time = 0.183

configs = ['Sequential\n(1 mapper)', 'Parallel\n(3 mappers)']
times = [sequential_time, parallel_time]
speedup = sequential_time / parallel_time

# Create bar chart
fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(12, 5))

# Time comparison
ax1.bar(configs, times, color=['#FF6B6B', '#4ECDC4'])
ax1.set_ylabel('Time (seconds)')
ax1.set_title('Processing Time Comparison')
ax1.set_ylim(0, max(times) * 1.2)
for i, v in enumerate(times):
    ax1.text(i, v + 0.01, f'{v:.3f}s', ha='center', va='bottom')

# Speedup
ax2.bar(['Speedup'], [speedup], color='#95E1D3', width=0.5)
ax2.set_ylabel('Speedup Factor')
ax2.set_title(f'Parallel Speedup: {speedup:.2f}x')
ax2.set_ylim(0, speedup * 1.2)
ax2.text(0, speedup + 0.05, f'{speedup:.2f}x', ha='center', va='bottom', fontsize=14, fontweight='bold')

plt.tight_layout()
plt.savefig('mapreduce_speedup.png', dpi=300, bbox_inches='tight')
print("Chart saved as mapreduce_speedup.png")
